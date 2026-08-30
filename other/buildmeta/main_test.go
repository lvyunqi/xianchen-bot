package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xianlv/internal/appinfo"
)

func TestReadPluginMetadataUsesSharedAppInfo(t *testing.T) {
	source := `package main
import "xianlv/internal/appinfo"
const (
	PluginName = appinfo.PluginName
	PluginAuthor = "随缘"
	PluginVersion = appinfo.Version
	PluginDescription = "说明"
)`
	metadata, err := readPluginMetadata(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Name != appinfo.PluginName || metadata.Version != appinfo.Version {
		t.Fatalf("metadata identity drifted: %+v", metadata)
	}
	if metadata.Author != "随缘" || metadata.Description != "说明" {
		t.Fatalf("literal metadata was not preserved: %+v", metadata)
	}
}

func TestHashWorkerFileUsesSHA256AndRejectsEmptyInput(t *testing.T) {
	workerPath := filepath.Join(t.TempDir(), "bee_go_worker.exe")
	if err := os.WriteFile(workerPath, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := hashWorkerFile(workerPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := hex.EncodeToString(digest), "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"; got != want {
		t.Fatalf("worker SHA-256=%s want=%s", got, want)
	}
	emptyPath := filepath.Join(t.TempDir(), "empty-worker.exe")
	if err := os.WriteFile(emptyPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := hashWorkerFile(emptyPath); err == nil {
		t.Fatal("empty worker unexpectedly produced a publishable hash")
	}
}

func TestGeneratePluginHeaderEmbedsOnlyValidWorkerHash(t *testing.T) {
	metadata := pluginMetadata{Name: "仙尘", Author: "随缘", Version: "2.2.1", Description: "说明"}
	digest, err := hex.DecodeString("ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")
	if err != nil {
		t.Fatal(err)
	}
	header, err := generatePluginHeader(metadata, digest)
	if err != nil {
		t.Fatal(err)
	}
	text := string(header)
	if !strings.Contains(text, "#define BEE_WORKER_SHA256_VALID 1") || !strings.Contains(text, "#define BEE_WORKER_SHA256_SIZE 32") {
		t.Fatalf("valid worker hash marker missing:\n%s", text)
	}
	for _, value := range digest {
		if !strings.Contains(text, fmt.Sprintf("0x%02x,", value)) {
			t.Fatalf("worker digest byte 0x%02x missing from header", value)
		}
	}
	placeholder, err := generatePluginHeader(metadata, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(placeholder), "#define BEE_WORKER_SHA256_VALID 0") {
		t.Fatal("first-stage metadata did not mark the worker hash invalid")
	}
	if _, err := generatePluginHeader(metadata, make([]byte, 31)); err == nil {
		t.Fatal("malformed worker hash unexpectedly generated a header")
	}
}

func TestBuildChainRegeneratesHashHeaderAndLinksBCrypt(t *testing.T) {
	project := filepath.Clean(filepath.Join("..", ".."))
	assertFileContains := func(path string, fragments ...string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(project, path))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		for _, fragment := range fragments {
			if !strings.Contains(text, fragment) {
				t.Fatalf("%s missing %q", path, fragment)
			}
		}
		return text
	}
	assertFileContains("build-x86.ps1",
		"go run ./other/buildmeta plugin_main.go temp temp/bee_go_worker.exe",
		"#define BEE_WORKER_SHA256_VALID 1",
		"-lbcrypt",
	)
	assertFileContains("build.bat",
		`go run .\other\buildmeta plugin_main.go temp temp\bee_go_worker.exe`,
		"#define BEE_WORKER_SHA256_VALID 1",
		"-lbcrypt",
	)
	bridge := assertFileContains(filepath.Join("other", "bee_bridge.c"),
		"BCryptOpenAlgorithmProvider",
		"BCryptHashData",
		"BCryptFinishHash",
		"verify_worker_resource(bytes, size)",
		"refusing to build a publishable DLL without a valid Worker SHA-256",
	)
	verifyAt := strings.Index(bridge, "if (!verify_worker_resource(bytes, size)) return FALSE;")
	createAt := strings.Index(bridge, "file = CreateFileW(g_worker_path")
	if verifyAt < 0 || createAt < 0 || verifyAt >= createAt {
		t.Fatal("Worker resource is not verified before the executable file is created")
	}
}
