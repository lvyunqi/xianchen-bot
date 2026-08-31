use std::env;
use std::fs;
use std::path::PathBuf;

fn main() {
    println!("cargo:rerun-if-env-changed=XIANCHEN_WORKER_BIN");
    let manifest = PathBuf::from(env::var_os("CARGO_MANIFEST_DIR").expect("CARGO_MANIFEST_DIR"));
    let source = env::var_os("XIANCHEN_WORKER_BIN")
        .map(PathBuf::from)
        .unwrap_or_else(|| manifest.join("embedded").join(worker_file_name()));
    println!("cargo:rerun-if-changed={}", source.display());
    let metadata = fs::metadata(&source).unwrap_or_else(|error| {
        panic!(
            "未找到 Go worker {}：请先由流水线构建，或设置 XIANCHEN_WORKER_BIN（{error}）",
            source.display()
        )
    });
    assert!(metadata.len() > 0, "Go worker 文件为空：{}", source.display());
    let output = PathBuf::from(env::var_os("OUT_DIR").expect("OUT_DIR")).join("xianchen-worker");
    fs::copy(&source, &output).unwrap_or_else(|error| {
        panic!(
            "复制 Go worker {} -> {} 失败：{error}",
            source.display(),
            output.display()
        )
    });
}

fn worker_file_name() -> &'static str {
    if cfg!(windows) {
        "xianchen-worker.exe"
    } else {
        "xianchen-worker"
    }
}
