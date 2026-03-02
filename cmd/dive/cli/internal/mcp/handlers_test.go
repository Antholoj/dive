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

func TestHandlers_ResourceSummary(t *testing.T) {
	h := newToolHandlers()
	h.analyses["docker:ubuntu:latest"] = &image.Analysis{
		Image:       "ubuntu:latest",
		WastedBytes: 0,
		SizeBytes:   100,
		Efficiency:  1.0,
	}

	ctx := context.Background()
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "dive://image/ubuntu:latest/summary"

	result, err := h.resourceSummaryHandler(ctx, req)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	textRes, ok := result[0].(mcp.TextResourceContents)
	assert.True(t, ok)
	assert.Contains(t, textRes.Text, "Image: ubuntu:latest")
}

func TestHandlers_PromptOptimize(t *testing.T) {
	h := newToolHandlers()
	h.analyses["docker:ubuntu:latest"] = &image.Analysis{
		Image:       "ubuntu:latest",
		WastedBytes: 0,
		SizeBytes:   100,
		Efficiency:  1.0,
	}

	ctx := context.Background()
	req := mcp.GetPromptRequest{}
	req.Params.Name = "optimize-dockerfile"
	req.Params.Arguments = map[string]string{
		"image": "ubuntu:latest",
	}

	result, err := h.promptOptimizeDockerfileHandler(ctx, req)
	assert.NoError(t, err)
	assert.Contains(t, result.Description, "Optimize Dockerfile")
	assert.Len(t, result.Messages, 1)
	assert.Contains(t, result.Messages[0].Content.(mcp.TextContent).Text, "ubuntu:latest")
}
