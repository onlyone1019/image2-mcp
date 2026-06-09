package image2

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.schyler.top"
	DefaultModel   = "gpt-image-2"
	DefaultSize    = "1024x1024"
)

var safeNameRE = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type Client struct {
	apiKey     string
	endpoint   string
	outputDir  string
	httpClient *http.Client
}

type GenerateRequest struct {
	Prompt     string `json:"prompt"`
	Size       string `json:"size,omitempty"`
	OutputDir  string `json:"output_dir,omitempty"`
	OutputName string `json:"output_name,omitempty"`
}

type GenerateResult struct {
	FilePath string `json:"file_path"`
	Model    string `json:"model"`
	Size     string `json:"size"`
}

func NewFromEnv(outputDir string) (*Client, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_IMAGE_API_KEY"))
	if apiKey == "" {
		return nil, errors.New("OPENAI_IMAGE_API_KEY is required")
	}
	baseURL := strings.TrimSpace(os.Getenv("OPENAI_IMAGE_BASE_URL"))
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		apiKey:    apiKey,
		endpoint:  BuildGenerationsEndpoint(baseURL),
		outputDir: outputDir,
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
		},
	}, nil
}

func New(apiKey, baseURL, outputDir string, httpClient *http.Client) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("OPENAI_IMAGE_API_KEY is required")
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 180 * time.Second}
	}
	return &Client{
		apiKey:     apiKey,
		endpoint:   BuildGenerationsEndpoint(baseURL),
		outputDir:  outputDir,
		httpClient: httpClient,
	}, nil
}

func BuildGenerationsEndpoint(baseURL string) string {
	u := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(u, "/v1/images/generations") {
		return u
	}
	if strings.HasSuffix(u, "/images/generations") {
		return u
	}
	if strings.HasSuffix(u, "/v1") {
		return u + "/images/generations"
	}
	return u + "/v1/images/generations"
}

func (c *Client) Generate(ctx context.Context, input GenerateRequest) (GenerateResult, error) {
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return GenerateResult{}, errors.New("prompt is required")
	}
	size := strings.TrimSpace(input.Size)
	if size == "" {
		size = DefaultSize
	}
	outputDir := strings.TrimSpace(input.OutputDir)
	if outputDir == "" {
		outputDir = c.outputDir
	} else if !filepath.IsAbs(outputDir) {
		return GenerateResult{}, errors.New("output_dir must be an absolute path")
	}

	payload := map[string]any{
		"model":  DefaultModel,
		"prompt": prompt,
		"size":   size,
		"n":      1,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return GenerateResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return GenerateResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("request image generation: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return GenerateResult{}, fmt.Errorf("read image response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return GenerateResult{}, fmt.Errorf("image API returned HTTP %d: %s", resp.StatusCode, summarize(respBody))
	}

	var parsed struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
		Error any `json:"error,omitempty"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return GenerateResult{}, fmt.Errorf("parse image response: %w: %s", err, summarize(respBody))
	}
	if len(parsed.Data) == 0 || strings.TrimSpace(parsed.Data[0].B64JSON) == "" {
		return GenerateResult{}, fmt.Errorf("image response missing data[0].b64_json: %s", summarize(respBody))
	}

	pngBytes, err := base64.StdEncoding.DecodeString(parsed.Data[0].B64JSON)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("decode data[0].b64_json: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return GenerateResult{}, fmt.Errorf("create output directory: %w", err)
	}

	fileName := cleanOutputName(input.OutputName)
	if fileName == "" {
		fileName = "image2-" + time.Now().Format("20060102-150405") + ".png"
	}
	filePath := filepath.Join(outputDir, fileName)
	if err := os.WriteFile(filePath, pngBytes, 0o644); err != nil {
		return GenerateResult{}, fmt.Errorf("write PNG: %w", err)
	}

	return GenerateResult{
		FilePath: filePath,
		Model:    DefaultModel,
		Size:     size,
	}, nil
}

func cleanOutputName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = filepath.Base(name)
	name = safeNameRE.ReplaceAllString(name, "-")
	name = strings.Trim(name, ".-")
	if name == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(name), ".png") {
		name += ".png"
	}
	return name
}

func summarize(body []byte) string {
	const max = 800
	s := strings.TrimSpace(string(body))
	if len(s) > max {
		return s[:max] + "...(truncated)"
	}
	return s
}
