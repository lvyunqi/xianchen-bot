param(
    [string]$OutputName = "仙尘.dll",
    [string]$ZigPath = $env:ZIG_EXE
)

$ErrorActionPreference = "Stop"
$Project = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Project

$zigFallback = "C:\Users\Administrator\Downloads\zig-x86_64-windows-0.17.0-dev.1417+20befa4e6\zig-x86_64-windows-0.17.0-dev.1417+20befa4e6\zig.exe"
if ([string]::IsNullOrWhiteSpace($ZigPath)) {
    $zigCommand = Get-Command zig.exe -CommandType Application -ErrorAction SilentlyContinue
    if ($zigCommand) {
        $ZigPath = $zigCommand.Source
    }
    elseif (Test-Path -LiteralPath $zigFallback -PathType Leaf) {
        $ZigPath = $zigFallback
    }
}
if ([string]::IsNullOrWhiteSpace($ZigPath) -or -not (Test-Path -LiteralPath $ZigPath -PathType Leaf)) {
    throw "未找到 zig.exe，请安装 Zig、加入 PATH，或通过 -ZigPath / ZIG_EXE 指定路径。"
}
$ZigPath = (Resolve-Path -LiteralPath $ZigPath).Path
& $ZigPath version | Out-Null
if ($LASTEXITCODE -ne 0) { throw "zig.exe 无法运行：$ZigPath" }

$tempPath = Join-Path $Project "temp"
$buildPath = Join-Path $Project "build"
if (Test-Path -LiteralPath $tempPath) {
    $resolvedTemp = (Resolve-Path -LiteralPath $tempPath).Path
    if (-not $resolvedTemp.StartsWith($Project, [StringComparison]::OrdinalIgnoreCase)) {
        throw "临时目录越界：$resolvedTemp"
    }
    Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
}
New-Item -ItemType Directory -Path $tempPath -Force | Out-Null
New-Item -ItemType Directory -Path $buildPath -Force | Out-Null

try {
    Write-Host "[1/5] 生成插件元数据"
    go run ./other/buildmeta plugin_main.go temp | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "元数据生成失败" }

    Write-Host "[2/5] 编译 Windows x86 Go Worker"
    $env:GOOS = "windows"
    $env:GOARCH = "386"
    $env:CGO_ENABLED = "1"
    $env:CC = '"' + $ZigPath + '" cc -target x86-windows-gnu'
    go build -buildvcs=false -trimpath -ldflags="-s -w -buildid= -extldflags=-static" -o temp/bee_go_worker.exe .
    if ($LASTEXITCODE -ne 0) { throw "Worker 编译失败" }
    Remove-Item -LiteralPath (Join-Path $Project "worker_runtime.go") -Force -ErrorAction SilentlyContinue

    Write-Host "[3/5] 绑定 Worker SHA-256"
    go run ./other/buildmeta plugin_main.go temp temp/bee_go_worker.exe | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Worker SHA-256 元数据生成失败" }
    $pluginHeader = Get-Content -LiteralPath (Join-Path $tempPath "plugin_config.h") -Raw
    if ($pluginHeader -notmatch '(?m)^#define BEE_WORKER_SHA256_VALID 1$') {
        throw "Worker SHA-256 无效，拒绝发布构建"
    }

    Write-Host "[4/5] 嵌入 Worker 资源"
    Push-Location $tempPath
    try { & $ZigPath rc /c 65001 /fo worker.res worker.rc } finally { Pop-Location }
    if ($LASTEXITCODE -ne 0) { throw "资源编译失败" }

    Write-Host "[5/5] 链接 Bee PE32 DLL"
    $baseName = [IO.Path]::GetFileNameWithoutExtension($OutputName)
    & $ZigPath cc -target x86-windows-gnu -O2 -shared other/bee_bridge.c temp/worker.res other/BeePlugin.def -Itemp -lbcrypt -lkernel32 -o "build/$baseName.dll"
    if ($LASTEXITCODE -ne 0) { throw "DLL 链接失败" }

    Get-ChildItem -LiteralPath $buildPath -Include *.lib,*.pdb -File -ErrorAction SilentlyContinue | Remove-Item -Force
    Write-Host "构建成功：build/$baseName.dll"
}
finally {
    Remove-Item -LiteralPath (Join-Path $Project "worker_runtime.go") -Force -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $tempPath) {
        Remove-Item -LiteralPath $tempPath -Recurse -Force
    }
}
