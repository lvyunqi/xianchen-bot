package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"xianlv/internal/appinfo"
)

const workerResourceID = 101

type pluginMetadata struct {
	Name, Author, Version, Description string
}

type pluginInfo struct {
	Name, Author, Ver, Text string
}

func (info pluginInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name   string `json:"name"`
		Author string `json:"author"`
		Ver    string `json:"ver"`
		Text   string `json:"text"`
	}{info.Name, info.Author, info.Ver, info.Text})
}

func readPluginMetadata(reader io.Reader) (pluginMetadata, error) {
	source, err := io.ReadAll(reader)
	if err != nil {
		return pluginMetadata{}, err
	}
	file, err := parser.ParseFile(token.NewFileSet(), "plugin_main.go", source, 0)
	if err != nil {
		return pluginMetadata{}, err
	}
	values := map[string]string{}
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for index, name := range spec.Names {
			if index >= len(spec.Values) {
				continue
			}
			literal, ok := spec.Values[index].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				continue
			}
			value, unquoteErr := strconv.Unquote(literal.Value)
			if unquoteErr == nil {
				values[name.Name] = value
			}
		}
		return true
	})
	// Plugin identity and version live in appinfo so the runtime page, data pack,
	// worker metadata and DLL metadata cannot drift apart.
	if values["PluginName"] == "" {
		values["PluginName"] = appinfo.PluginName
	}
	if values["PluginVersion"] == "" {
		values["PluginVersion"] = appinfo.Version
	}
	metadata := pluginMetadata{values["PluginName"], values["PluginAuthor"], values["PluginVersion"], values["PluginDescription"]}
	if metadata.Name == "" || metadata.Author == "" || metadata.Version == "" {
		return pluginMetadata{}, errors.New("plugin_main.go missing PluginName/PluginAuthor/PluginVersion string constants")
	}
	return metadata, nil
}

func encodeGBK(value []byte) ([]byte, error) {
	encoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), value)
	return encoded, err
}

func cByteArray(name string, values []byte) string {
	var output strings.Builder
	fmt.Fprintf(&output, "static const unsigned char %s[] = {\n    ", name)
	for index, value := range values {
		if index > 0 {
			if index%12 == 0 {
				output.WriteString("\n    ")
			} else {
				output.WriteString(" ")
			}
		}
		fmt.Fprintf(&output, "0x%02x,", value)
	}
	output.WriteString("\n};\n")
	return output.String()
}

func hashWorkerFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open worker for hashing: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat worker for hashing: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 {
		return nil, errors.New("worker hash input must be a non-empty regular file")
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return nil, fmt.Errorf("hash worker: %w", err)
	}
	return digest.Sum(nil), nil
}

func generatePluginHeader(metadata pluginMetadata, workerHash []byte) ([]byte, error) {
	if workerHash != nil && len(workerHash) != sha256.Size {
		return nil, fmt.Errorf("worker SHA-256 must contain exactly %d bytes", sha256.Size)
	}
	jsonData, err := json.Marshal(pluginInfo{metadata.Name, metadata.Author, metadata.Version, metadata.Description})
	if err != nil {
		return nil, err
	}
	gbkJSON, err := encodeGBK(jsonData)
	if err != nil {
		return nil, fmt.Errorf("plugin metadata cannot be encoded as GBK: %w", err)
	}
	gbkJSON = append(gbkJSON, 0)
	units := append(utf16.Encode([]rune(metadata.Name)), 0)
	var output bytes.Buffer
	output.WriteString("#ifndef BEE_PLUGIN_CONFIG_H\n#define BEE_PLUGIN_CONFIG_H\n\n#include <windows.h>\n\n")
	fmt.Fprintf(&output, "#define BEE_WORKER_RESOURCE_ID %d\n#define BEE_WORKER_FILENAME L\"bee_go_worker.exe\"\n\n", workerResourceID)
	workerHashValid := 0
	if len(workerHash) == sha256.Size {
		workerHashValid = 1
	} else {
		workerHash = make([]byte, sha256.Size)
	}
	fmt.Fprintf(&output, "#define BEE_WORKER_SHA256_VALID %d\n#define BEE_WORKER_SHA256_SIZE %d\n", workerHashValid, sha256.Size)
	output.WriteString(cByteArray("BEE_WORKER_SHA256", workerHash))
	output.WriteString("\n")
	output.WriteString("static const WCHAR BEE_PLUGIN_NAME_W[] = {\n    ")
	for index, unit := range units {
		if index > 0 {
			output.WriteString(", ")
		}
		fmt.Fprintf(&output, "0x%04x", unit)
	}
	output.WriteString("\n};\n\n")
	output.WriteString(cByteArray("BEE_INIT_JSON_GBK", gbkJSON))
	output.WriteString("\n#endif\n")
	return output.Bytes(), nil
}

func generateWorkerRC() []byte {
	return []byte(fmt.Sprintf("%d RCDATA \"bee_go_worker.exe\"\r\n", workerResourceID))
}

func materializeWorkerRuntime(projectDir string) error {
	sourcePath := filepath.Join(projectDir, "other", "worker_runtime.go")
	targetPath := filepath.Join(projectDir, "worker_runtime.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read worker runtime: %w", err)
	}
	const buildTag = "//go:build bee_worker_runtime\n\n"
	if !bytes.HasPrefix(source, []byte(buildTag)) {
		return errors.New("other/worker_runtime.go missing bee_worker_runtime build tag")
	}
	if err := os.WriteFile(targetPath, bytes.TrimPrefix(source, []byte(buildTag)), 0o644); err != nil {
		return fmt.Errorf("materialize worker runtime: %w", err)
	}
	return nil
}

func main() {
	if len(os.Args) != 3 && len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: go run ./other/buildmeta <plugin_main.go> <output-dir> [worker-executable]")
		os.Exit(2)
	}
	source, err := os.Open(os.Args[1])
	if err != nil {
		fail(err)
	}
	metadata, err := readPluginMetadata(source)
	source.Close()
	if err != nil {
		fail(err)
	}
	var workerHash []byte
	if len(os.Args) == 4 {
		workerHash, err = hashWorkerFile(os.Args[3])
		if err != nil {
			fail(err)
		}
	}
	header, err := generatePluginHeader(metadata, workerHash)
	if err != nil {
		fail(err)
	}
	if err := os.MkdirAll(os.Args[2], 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(os.Args[2], "plugin_config.h"), header, 0o644); err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(os.Args[2], "worker.rc"), generateWorkerRC(), 0o644); err != nil {
		fail(err)
	}
	if len(os.Args) == 3 {
		if err := materializeWorkerRuntime(filepath.Dir(os.Args[1])); err != nil {
			fail(err)
		}
	}
	gbkName, err := encodeGBK([]byte(metadata.Name))
	if err != nil {
		fail(fmt.Errorf("plugin name cannot be encoded as GBK: %w", err))
	}
	if _, err := os.Stdout.Write(append(gbkName, '\r', '\n')); err != nil {
		fail(err)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, "metadata generation failed:", err); os.Exit(1) }
