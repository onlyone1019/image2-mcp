package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerListsGenerateImage2AndEditImage2Tools(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	server := newServer(root, t.TempDir())
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)

	t1, t2 := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()

	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	var names []string
	for tool, err := range clientSession.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, tool.Name)
	}
	if len(names) != 2 {
		t.Fatalf("tools = %v, want [edit_image2 generate_image2]", names)
	}
	want := map[string]bool{"generate_image2": true, "edit_image2": true}
	for _, n := range names {
		if !want[n] {
			t.Fatalf("unexpected tool %q", n)
		}
	}
}

func TestGenerateImage2ToolReportsMissingAPIKey(t *testing.T) {
	t.Setenv("OPENAI_IMAGE_API_KEY", "")
	t.Setenv("OPENAI_IMAGE_BASE_URL", "https://api.schyler.top")

	ctx := context.Background()
	root := t.TempDir()
	server := newServer(root, t.TempDir())
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)

	t1, t2 := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()

	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "generate_image2",
		Arguments: generateParams{
			Prompt: "test",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for missing OPENAI_IMAGE_API_KEY")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected error content")
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(text.Text, "OPENAI_IMAGE_API_KEY is required") {
		t.Fatalf("unexpected error content: %#v", result.Content)
	}
}

func TestEditImage2ToolReportsMissingAPIKey(t *testing.T) {
	t.Setenv("OPENAI_IMAGE_API_KEY", "")
	t.Setenv("OPENAI_IMAGE_BASE_URL", "https://api.schyler.top")

	ctx := context.Background()
	root := t.TempDir()
	server := newServer(root, t.TempDir())
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)

	t1, t2 := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()

	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "edit_image2",
		Arguments: editParams{
			Prompt:     "edit test",
			ImagePaths: []string{"/tmp/test.png"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for missing OPENAI_IMAGE_API_KEY")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected error content")
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(text.Text, "OPENAI_IMAGE_API_KEY is required") {
		t.Fatalf("unexpected error content: %#v", result.Content)
	}
}

func TestEditImage2ToolReportsMissingImages(t *testing.T) {
	t.Setenv("OPENAI_IMAGE_API_KEY", "test-key")

	ctx := context.Background()
	root := t.TempDir()
	server := newServer(root, t.TempDir())
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)

	t1, t2 := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()

	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "edit_image2",
		Arguments: editParams{Prompt: "edit test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for missing images")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected error content")
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(text.Text, "at least one image path is required") {
		t.Fatalf("unexpected error content: %#v", result.Content)
	}
}

func TestEditImage2ToolMapsQualityAndMask(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	inputDir := t.TempDir()
	imagePath := filepath.Join(inputDir, "reference.png")
	maskPath := filepath.Join(inputDir, "mask.png")
	for _, path := range []string{imagePath, maskPath} {
		if err := os.WriteFile(path, png, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/edits" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("quality"); got != "high" {
			t.Fatalf("quality = %q, want high", got)
		}
		images := r.MultipartForm.File["image[]"]
		if len(images) != 1 || images[0].Filename != "reference.png" {
			t.Fatalf("image files = %#v", images)
		}
		masks := r.MultipartForm.File["mask"]
		if len(masks) != 1 || masks[0].Filename != "mask.png" {
			t.Fatalf("mask files = %#v", masks)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(png)}},
		})
	}))
	defer apiServer.Close()

	t.Setenv("OPENAI_IMAGE_API_KEY", "test-key")
	t.Setenv("OPENAI_IMAGE_BASE_URL", apiServer.URL)
	outDir := t.TempDir()
	ctx := context.Background()
	server := newServer(t.TempDir(), t.TempDir())
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	t1, t2 := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "edit_image2",
		Arguments: editParams{
			Prompt:     "make it cinematic",
			ImagePaths: []string{imagePath},
			MaskPath:   maskPath,
			Quality:    "high",
			OutputDir:  outDir,
			OutputName: "edited.png",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %#v", result.Content)
	}
	if _, err := os.Stat(filepath.Join(outDir, "edited.png")); err != nil {
		t.Fatal(err)
	}
}
