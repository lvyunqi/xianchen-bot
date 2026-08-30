@echo off
setlocal EnableExtensions DisableDelayedExpansion
cd /d "%~dp0"
chcp 936 >nul

echo ========================================
echo Bee Go plugin Windows x86 build
echo ========================================
echo.

where go >nul 2>nul || goto :missing_go
where zig >nul 2>nul || goto :missing_zig

if exist temp rmdir /s /q temp
mkdir temp || goto :fail
if not exist build mkdir build || goto :fail

echo [1/5] Generating plugin metadata...
go run .\other\buildmeta plugin_main.go temp > temp\plugin_name.txt
if errorlevel 1 goto :fail
set /p "PLUGIN_NAME=" < temp\plugin_name.txt
if not defined PLUGIN_NAME goto :metadata_empty
del /q temp\plugin_name.txt >nul 2>nul

set "DLL_NAME=%~1"
if not defined DLL_NAME set "DLL_NAME=%PLUGIN_NAME%"
if /i "%DLL_NAME:~-4%"==".dll" set "DLL_NAME=%DLL_NAME:~0,-4%"
if not defined DLL_NAME goto :invalid_name

echo [2/5] Building Windows x86 Go Worker...
set GOOS=windows
set GOARCH=386
set CGO_ENABLED=0
go build -buildvcs=false -trimpath -ldflags="-s -w" -o temp\bee_go_worker.exe .
set "BUILD_RESULT=%ERRORLEVEL%"
del /q worker_runtime.go >nul 2>nul
if not "%BUILD_RESULT%"=="0" goto :fail

echo [3/5] Binding Worker SHA-256...
go run .\other\buildmeta plugin_main.go temp temp\bee_go_worker.exe >nul
if errorlevel 1 goto :invalid_hash
findstr /x /c:"#define BEE_WORKER_SHA256_VALID 1" temp\plugin_config.h >nul
if errorlevel 1 goto :invalid_hash

echo [4/5] Compiling embedded Worker resource...
pushd temp
zig rc /c 65001 /fo worker.res worker.rc
if errorlevel 1 (popd & goto :fail)
popd

echo [5/5] Linking Bee PE32 DLL...
zig cc -target x86-windows-gnu -O2 -shared other\bee_bridge.c temp\worker.res other\BeePlugin.def -Itemp -lbcrypt -lkernel32 -o "build\%DLL_NAME%.dll" || goto :fail

del /q build\*.lib build\*.pdb >nul 2>nul
rmdir /s /q temp
echo.
echo Build succeeded: build\%DLL_NAME%.dll
pause
exit /b 0

:missing_go
echo ERROR: Go was not found on PATH.
goto :error_exit

:missing_zig
echo ERROR: Zig was not found on PATH.
goto :error_exit

:metadata_empty
echo ERROR: Plugin metadata returned an empty plugin name.
goto :fail

:invalid_name
echo ERROR: DLL file name is invalid.
goto :fail

:invalid_hash
echo ERROR: Worker SHA-256 metadata is missing or invalid. Build refused.
goto :fail

:fail
if exist worker_runtime.go del /q worker_runtime.go >nul 2>nul
if exist temp rmdir /s /q temp
echo ERROR: Build failed. Review the compiler output above.

:error_exit
echo.
pause
exit /b 1
