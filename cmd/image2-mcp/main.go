package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"image2-mcp/internal/image2"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type generateParams struct {
	Prompt     string `json:"prompt" jsonschema:"Image prompt to generate."`
	Size       string `json:"size,omitempty" jsonschema:"Image size, defaults to 1024x1024."`
	OutputDir  string `json:"output_dir,omitempty" jsonschema:"Optional absolute directory to save the PNG."`
	OutputName string `json:"output_name,omitempty" jsonschema:"Optional PNG file name. Defaults to image2-{timestamp}.png."`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return err
	}
	outputDir := filepath.Join(projectRoot, "output", "imagegen")
	server := newServer(projectRoot, outputDir)
	return server.Run(context.Background(), &mcp.StdioTransport{})
}

func newServer(projectRoot, outputDir string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "image2-mcp",
		Version: "0.1.0",
	}, &mcp.ServerOptions{
		Instructions: "Generate images with gpt-image-2 via OPENAI_IMAGE_BASE_URL and OPENAI_IMAGE_API_KEY. The generate_image2 tool writes PNG files to output_dir when provided, otherwise output/imagegen, and returns the local file path.",
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "generate_image2",
		Description: "Generate one PNG image using gpt-image-2 and save it locally.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params generateParams) (*mcp.CallToolResult, image2.GenerateResult, error) {
		client, err := image2.NewFromEnv(outputDir)
		if err != nil {
			return nil, image2.GenerateResult{}, err
		}
		result, err := client.Generate(ctx, image2.GenerateRequest{
			Prompt:     params.Prompt,
			Size:       params.Size,
			OutputDir:  params.OutputDir,
			OutputName: params.OutputName,
		})
		if err != nil {
			return nil, image2.GenerateResult{}, err
		}
		text, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, image2.GenerateResult{}, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(text)},
			},
		}, result, nil
	})

	return server
}

func findProjectRoot() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	dir := filepath.Dir(exe)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return wd, nil
}
