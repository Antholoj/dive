package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/wagoodman/dive/dive/image"
)

func TestHandlers_AnalyzeImage_MissingImage(t *testing.T) {
	h := newToolHandlers()
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

	h := newToolHandlers()
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
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Top Inefficient Files")
}

func TestHandlers_GetWastedSpace_WithCache(t *testing.T) {
	h := newToolHandlers()
	h.analyses["docker:ubuntu:latest"] = &image.Analysis{
		Image:       "ubuntu:latest",
		WastedBytes: 0,
	}

	ctx := context.Background()
	req := mcp.CallToolRequest{}
	req.Params.Name = "get_wasted_space"
	req.Params.Arguments = map[string]interface{}{
		"image": "ubuntu:latest",
	}

	result, err := h.getWastedSpaceHandler(ctx, req)
	assert.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "No wasted space detected")
}
