package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/wagoodman/dive/cmd/dive/cli/internal/options"
	"github.com/wagoodman/dive/dive/image"
)

func TestHandlers_AnalyzeImage_MissingImage(t *testing.T) {
	h := newToolHandlers(options.DefaultMCP())
	ctx := context.Background()
	req := mcp.CallToolRequest{}
	req.Params.Name = "analyze_image"
	req.Params.Arguments = map[string]interface{}{}

	result, err := h.analyzeImageHandler(ctx, req)
	assert.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestHandlers_GetWastedSpace_AutoAnalysis(t *testing.T) {
	wd, _ := os.Getwd()
	root := wd
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		root = filepath.Dir(root)
	}
	
	imagePath := filepath.Join(root, ".data/test-docker-image.tar")

	h := newToolHandlers(options.DefaultMCP())
	ctx := context.Background()
	req := mcp.CallToolRequest{}
	req.Params.Name = "get_wasted_space"
	req.Params.Arguments = map[string]interface{}{
		"image":  imagePath,
		"source": "docker-archive",
	}

	result, err := h.getWastedSpaceHandler(ctx, req)
	assert.NoError(t, err)
	assert.False(t, result.IsError)
	
	var wasted WastedSpaceResult
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &wasted)
	assert.NoError(t, err)
	assert.Contains(t, wasted.Image, "test-docker-image.tar")
	assert.NotEmpty(t, wasted.Inefficiencies)
}

func TestHandlers_SandboxViolation(t *testing.T) {
	opts := options.DefaultMCP()
	opts.Sandbox = "/tmp/nothing"
	h := newToolHandlers(opts)
	ctx := context.Background()
	req := mcp.CallToolRequest{}
	req.Params.Name = "analyze_image"
	req.Params.Arguments = map[string]interface{}{
		"image":  ".data/test-docker-image.tar",
		"source": "docker-archive",
	}

	result, err := h.analyzeImageHandler(ctx, req)
	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "security: path")
}

func TestHandlers_ResourceSummary(t *testing.T) {
	h := newToolHandlers(options.DefaultMCP())
	h.analyses.Add("docker:ubuntu:latest", &image.Analysis{
		Image:       "ubuntu:latest",
		WastedBytes: 0,
		SizeBytes:   100,
		Efficiency:  1.0,
	})

	ctx := context.Background()
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "dive://image/ubuntu:latest/summary"

	result, err := h.resourceSummaryHandler(ctx, req)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	textRes, ok := result[0].(mcp.TextResourceContents)
	assert.True(t, ok)
	
	var summary ImageSummary
	err = json.Unmarshal([]byte(textRes.Text), &summary)
	assert.NoError(t, err)
	assert.Equal(t, "ubuntu:latest", summary.Image)
}

func TestHandlers_DiffLayers(t *testing.T) {
	wd, _ := os.Getwd()
	root := wd
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		root = filepath.Dir(root)
	}
	
	imagePath := filepath.Join(root, ".data/test-docker-image.tar")

	h := newToolHandlers(options.DefaultMCP())
	ctx := context.Background()
	req := mcp.CallToolRequest{}
	req.Params.Name = "diff_layers"
	req.Params.Arguments = map[string]interface{}{
		"image":              imagePath,
		"source":             "docker-archive",
		"base_layer_index":   0,
		"target_layer_index": 1,
	}

	result, err := h.diffLayersHandler(ctx, req)
	assert.NoError(t, err)
	assert.False(t, result.IsError)
	
	var diff DiffResult
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &diff)
	assert.NoError(t, err)
	assert.Equal(t, 0, diff.BaseLayer)
	assert.Equal(t, 1, diff.TargetLayer)
	assert.NotEmpty(t, diff.Changes)
}
