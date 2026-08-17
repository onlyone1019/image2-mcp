package image2

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildGenerationsEndpoint(t *testing.T) {
	tests := map[string]string{
		"https://api.schyler.top":                       "https://api.schyler.top/v1/images/generations",
		"https://api.schyler.top/":                      "https://api.schyler.top/v1/images/generations",
		"https://api.schyler.top/v1":                    "https://api.schyler.top/v1/images/generations",
		"https://api.schyler.top/v1/":                   "https://api.schyler.top/v1/images/generations",
		"https://api.schyler.top/images/edits":          "https://api.schyler.top/images/generations",
		"https://api.schyler.top/v1/images/edits":       "https://api.schyler.top/v1/images/generations",
		"https://api.schyler.top/v1/images/generations": "https://api.schyler.top/v1/images/generations",
	}
	for in, want := range tests {
		if got := BuildGenerationsEndpoint(in); got != want {
			t.Fatalf("BuildGenerationsEndpoint(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildEditsEndpoint(t *testing.T) {
	tests := map[string]string{
		"https://api.schyler.top":                       "https://api.schyler.top/v1/images/edits",
		"https://api.schyler.top/":                      "https://api.schyler.top/v1/images/edits",
		"https://api.schyler.top/v1":                    "https://api.schyler.top/v1/images/edits",
		"https://api.schyler.top/v1/":                   "https://api.schyler.top/v1/images/edits",
		"https://api.schyler.top/images/generations":    "https://api.schyler.top/images/edits",
		"https://api.schyler.top/images/edits":          "https://api.schyler.top/images/edits",
		"https://api.schyler.top/v1/images/edits":       "https://api.schyler.top/v1/images/edits",
		"https://api.schyler.top/v1/images/generations": "https://api.schyler.top/v1/images/edits",
	}
	for in, want := range tests {
		if got := BuildEditsEndpoint(in); got != want {
			t.Fatalf("BuildEditsEndpoint(%q) = %q, want %q", in, got, want)
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

func TestEditDecodesB64JSONAndWritesPNG(t *testing.T) {
	firstImage := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	secondImage := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	maskImage := append([]byte(nil), firstImage...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/edits" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "multipart/form-data") {
			t.Fatalf("Content-Type = %q, want multipart/form-data", ct)
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatal(err)
		}
		fields := map[string]string{}
		type uploadedFile struct {
			field       string
			name        string
			contentType string
			data        string
		}
		var files []uploadedFile
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatal(err)
			}
			if part.FileName() != "" {
				files = append(files, uploadedFile{
					field:       part.FormName(),
					name:        part.FileName(),
					contentType: part.Header.Get("Content-Type"),
					data:        string(data),
				})
				continue
			}
			fields[part.FormName()] = string(data)
		}
		wantFields := map[string]string{
			"model":           DefaultModel,
			"prompt":          "edit hello",
			"size":            DefaultSize,
			"quality":         "high",
			"n":               "1",
			"response_format": "b64_json",
		}
		for name, want := range wantFields {
			if got := fields[name]; got != want {
				t.Fatalf("field %q = %q, want %q", name, got, want)
			}
		}
		if len(files) != 3 {
			t.Fatalf("file parts = %d, want 3", len(files))
		}
		if files[0].field != "image[]" || files[0].name != "first.png" || files[0].contentType != "image/png" || files[0].data != string(firstImage) {
			t.Fatalf("first image part = %#v", files[0])
		}
		if files[1].field != "image[]" || files[1].name != "second.jpg" || files[1].contentType != "image/jpeg" || files[1].data != string(secondImage) {
			t.Fatalf("second image part = %#v", files[1])
		}
		if files[2].field != "mask" || files[2].name != "mask.png" || files[2].contentType != "image/png" || files[2].data != string(maskImage) {
			t.Fatalf("mask part = %#v", files[2])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(firstImage)}},
		})
	}))
	defer server.Close()

	imgDir := t.TempDir()
	paths := []struct {
		name string
		data []byte
	}{
		{name: "first.png", data: firstImage},
		{name: "second.jpg", data: secondImage},
		{name: "mask.png", data: maskImage},
	}
	for _, file := range paths {
		if err := os.WriteFile(filepath.Join(imgDir, file.name), file.data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	outDir := t.TempDir()
	client, err := New("test-key", server.URL, outDir, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Edit(context.Background(), EditRequest{
		Prompt:     "edit hello",
		ImagePaths: []string{filepath.Join(imgDir, "first.png"), filepath.Join(imgDir, "second.jpg")},
		MaskPath:   filepath.Join(imgDir, "mask.png"),
		Quality:    "high",
		OutputName: "edited.png",
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
	if string(got) != string(firstImage) {
		t.Fatalf("written bytes = %v, want %v", got, firstImage)
	}
}

func TestEditRequiresAtLeastOneImage(t *testing.T) {
	client, err := New("test-key", DefaultBaseURL, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Edit(context.Background(), EditRequest{Prompt: "hello", ImagePaths: nil})
	if err == nil || err.Error() != "at least one image path is required" {
		t.Fatalf("err = %v, want at least one image path is required", err)
	}
	_, err = client.Edit(context.Background(), EditRequest{Prompt: "hello", ImagePaths: []string{}})
	if err == nil || err.Error() != "at least one image path is required" {
		t.Fatalf("err = %v, want at least one image path is required", err)
	}
	_, err = client.Edit(context.Background(), EditRequest{Prompt: "hello", ImagePaths: []string{"   "}})
	if err == nil || err.Error() != "at least one image path is required" {
		t.Fatalf("err = %v, want at least one image path is required", err)
	}
}

func TestEditRejectsRelativeImagePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("HTTP server should not be called for relative image path")
	}))
	defer server.Close()

	client, err := New("test-key", server.URL, t.TempDir(), server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Edit(context.Background(), EditRequest{
		Prompt:     "hello",
		ImagePaths: []string{"relative/input.png"},
	})
	if err == nil || err.Error() != "image path must be an absolute path: relative/input.png" {
		t.Fatalf("err = %v, want image path must be an absolute path: relative/input.png", err)
	}
}

func TestEditRejectsMissingImageFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("HTTP server should not be called for missing image file")
	}))
	defer server.Close()

	client, err := New("test-key", server.URL, t.TempDir(), server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Edit(context.Background(), EditRequest{
		Prompt:     "hello",
		ImagePaths: []string{"/no/such/file.png"},
	})
	if err == nil || !strings.Contains(err.Error(), "image file not found") {
		t.Fatalf("err = %v, want image file not found", err)
	}
}

func TestEditRejectsDirectoryImagePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("HTTP server should not be called for directory image path")
	}))
	defer server.Close()

	directoryPath := t.TempDir()
	client, err := New("test-key", server.URL, t.TempDir(), server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Edit(context.Background(), EditRequest{
		Prompt:     "hello",
		ImagePaths: []string{directoryPath},
	})
	if err == nil || err.Error() != "image path must be a regular file: "+directoryPath {
		t.Fatalf("err = %v, want regular file error", err)
	}
}

func TestEditRejectsRelativeOutputDir(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("HTTP server should not be called for invalid output_dir")
	}))
	defer server.Close()

	imgDir := t.TempDir()
	imgPath := filepath.Join(imgDir, "input.png")
	if err := os.WriteFile(imgPath, []byte{0x89, 'P', 'N', 'G'}, 0o644); err != nil {
		t.Fatal(err)
	}

	client, err := New("test-key", server.URL, t.TempDir(), server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Edit(context.Background(), EditRequest{
		Prompt:     "hello",
		ImagePaths: []string{imgPath},
		OutputDir:  "relative/path",
	})
	if err == nil || err.Error() != "output_dir must be an absolute path" {
		t.Fatalf("err = %v, want output_dir must be an absolute path", err)
	}
}

func TestEditRejectsRelativeMaskPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("HTTP server should not be called for invalid mask_path")
	}))
	defer server.Close()

	imgDir := t.TempDir()
	imgPath := filepath.Join(imgDir, "input.png")
	if err := os.WriteFile(imgPath, []byte{0x89, 'P', 'N', 'G'}, 0o644); err != nil {
		t.Fatal(err)
	}

	client, err := New("test-key", server.URL, t.TempDir(), server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Edit(context.Background(), EditRequest{
		Prompt:     "hello",
		ImagePaths: []string{imgPath},
		MaskPath:   "relative/mask.png",
	})
	if err == nil || err.Error() != "mask_path must be an absolute path" {
		t.Fatalf("err = %v, want mask_path must be an absolute path", err)
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

func TestRealEditImage2Smoke(t *testing.T) {
	if os.Getenv("RUN_IMAGE2_EDIT_SMOKE") != "1" {
		t.Skip("set RUN_IMAGE2_EDIT_SMOKE=1 and IMAGE2_EDIT_INPUT=/absolute/path.png to call the real image edit API")
	}
	imagePath := os.Getenv("IMAGE2_EDIT_INPUT")
	if imagePath == "" {
		t.Fatal("IMAGE2_EDIT_INPUT is required")
	}
	outputDir, err := filepath.Abs(filepath.Join("..", "..", "output", "imagegen"))
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewFromEnv(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Edit(context.Background(), EditRequest{
		Prompt:     "Preserve the subject and composition, with polished cinematic lighting and natural photographic detail",
		ImagePaths: []string{imagePath},
		OutputName: "mcp-edit-smoke-test.png",
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

func TestRealEditImage2MultiSmoke(t *testing.T) {
	if os.Getenv("RUN_IMAGE2_EDIT_MULTI_SMOKE") != "1" {
		t.Skip("set RUN_IMAGE2_EDIT_MULTI_SMOKE=1 and IMAGE2_EDIT_INPUTS to call the real multi-image edit API")
	}
	imagePaths := filepath.SplitList(os.Getenv("IMAGE2_EDIT_INPUTS"))
	if len(imagePaths) < 2 {
		t.Fatal("IMAGE2_EDIT_INPUTS must contain at least two paths separated by the OS path-list separator")
	}
	outputDir, err := filepath.Abs(filepath.Join("..", "..", "output", "imagegen"))
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewFromEnv(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Edit(context.Background(), EditRequest{
		Prompt:     "Use the first image as the subject and composition reference, and the second image as the lighting, color, and styling reference",
		ImagePaths: imagePaths,
		OutputName: "mcp-edit-multi-smoke-test.png",
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
