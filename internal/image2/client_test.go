package image2

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildGenerationsEndpoint(t *testing.T) {
	tests := map[string]string{
		"https://api.schyler.top":                       "https://api.schyler.top/v1/images/generations",
		"https://api.schyler.top/":                      "https://api.schyler.top/v1/images/generations",
		"https://api.schyler.top/v1":                    "https://api.schyler.top/v1/images/generations",
		"https://api.schyler.top/v1/":                   "https://api.schyler.top/v1/images/generations",
		"https://api.schyler.top/v1/images/generations": "https://api.schyler.top/v1/images/generations",
	}
	for in, want := range tests {
		if got := BuildGenerationsEndpoint(in); got != want {
			t.Fatalf("BuildGenerationsEndpoint(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	if _, err := New("", DefaultBaseURL, t.TempDir(), nil); err == nil {
		t.Fatal("expected missing OPENAI_IMAGE_API_KEY error")
	}
}

func TestGenerateDecodesB64JSONAndWritesPNG(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req["model"] != DefaultModel || req["prompt"] != "hello" || req["size"] != DefaultSize || req["n"].(float64) != 1 {
			t.Fatalf("unexpected request payload: %#v", req)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(png)}},
		})
	}))
	defer server.Close()

	outDir := t.TempDir()
	client, err := New("test-key", server.URL, outDir, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Generate(context.Background(), GenerateRequest{
		Prompt:     "hello",
		OutputName: "../unsafe name",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Model != DefaultModel || result.Size != DefaultSize {
		t.Fatalf("unexpected result: %#v", result)
	}
	if filepath.Dir(result.FilePath) != outDir {
		t.Fatalf("file path escaped output dir: %s", result.FilePath)
	}
	got, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(png) {
		t.Fatalf("written bytes = %v, want %v", got, png)
	}
}

func TestGenerateUsesRequestedOutputDir(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(png)}},
		})
	}))
	defer server.Close()

	defaultDir := t.TempDir()
	requestedDir := filepath.Join(t.TempDir(), "custom")
	client, err := New("test-key", server.URL, defaultDir, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Generate(context.Background(), GenerateRequest{
		Prompt:     "hello",
		OutputDir:  requestedDir,
		OutputName: "custom.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(result.FilePath) != requestedDir {
		t.Fatalf("file dir = %q, want %q", filepath.Dir(result.FilePath), requestedDir)
	}
	if _, err := os.Stat(result.FilePath); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateRejectsRelativeOutputDir(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("HTTP server should not be called for invalid output_dir")
	}))
	defer server.Close()

	client, err := New("test-key", server.URL, t.TempDir(), server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Generate(context.Background(), GenerateRequest{
		Prompt:    "hello",
		OutputDir: "relative/path",
	})
	if err == nil || err.Error() != "output_dir must be an absolute path" {
		t.Fatalf("err = %v, want output_dir must be an absolute path", err)
	}
}

func TestRealGenerateImage2Smoke(t *testing.T) {
	if os.Getenv("RUN_IMAGE2_SMOKE") != "1" {
		t.Skip("set RUN_IMAGE2_SMOKE=1 to call the real image API")
	}
	client, err := NewFromEnv(filepath.Join("..", "..", "output", "imagegen"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Generate(context.Background(), GenerateRequest{
		Prompt:     "A compact clean desk setup for MCP smoke testing, realistic photo, soft studio light",
		OutputName: "mcp-smoke-test.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FilePath == "" {
		t.Fatal("expected output file path")
	}
	if _, err := os.Stat(result.FilePath); err != nil {
		t.Fatal(err)
	}
}
