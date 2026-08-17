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

type editParams struct {
	Prompt     string   `json:"prompt" jsonschema:"Image prompt for the edit."`
	ImagePaths []string `json:"image_paths" jsonschema:"Absolute paths to source images, preserved in request order. At least one required."`
	Size       string   `json:"size,omitempty" jsonschema:"Image size, defaults to 1024x1024."`
	Quality    string   `json:"quality,omitempty" jsonschema:"Image quality, defaults to auto."`
	OutputDir  string   `json:"output_dir,omitempty" jsonschema:"Optional absolute directory to save the PNG."`
	OutputName string   `json:"output_name,omitempty" jsonschema:"Optional PNG file name. Defaults to image2-{timestamp}.png."`
	MaskPath   string   `json:"mask_path,omitempty" jsonschema:"Optional absolute path to a mask PNG."`
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
		Version: "0.2.0",
	}, &mcp.ServerOptions{
		Instructions: "Generate and edit images with gpt-image-2 via OPENAI_IMAGE_BASE_URL and OPENAI_IMAGE_API_KEY. The generate_image2 tool generates and the edit_image2 tool edits PNG files, saving to output_dir when provided, otherwise output/imagegen, and returns the local file path.",
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

	mcp.AddTool(server, &mcp.Tool{
		Name:        "edit_image2",
		Description: "Edit one or more local images using gpt-image-2 with an optional mask and save the result locally.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params editParams) (*mcp.CallToolResult, image2.EditResult, error) {
		client, err := image2.NewFromEnv(outputDir)
		if err != nil {
			return nil, image2.EditResult{}, err
		}
		result, err := client.Edit(ctx, image2.EditRequest{
			Prompt:     params.Prompt,
			Size:       params.Size,
			Quality:    params.Quality,
			OutputDir:  params.OutputDir,
			OutputName: params.OutputName,
			ImagePaths: params.ImagePaths,
			MaskPath:   params.MaskPath,
		})
		if err != nil {
			return nil, image2.EditResult{}, err
		}
		text, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, image2.EditResult{}, err
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
