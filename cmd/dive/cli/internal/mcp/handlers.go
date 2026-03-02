package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/wagoodman/dive/cmd/dive/cli/internal/command/adapter"
	"github.com/wagoodman/dive/dive"
	"github.com/wagoodman/dive/dive/image"
	"github.com/wagoodman/dive/internal/log"
)

type toolHandlers struct {
	mu       sync.RWMutex
	analyses map[string]*image.Analysis
}

func newToolHandlers() *toolHandlers {
	return &toolHandlers{
		analyses: make(map[string]*image.Analysis),
	}
}

func (h *toolHandlers) getAnalysis(ctx context.Context, imageName string, sourceStr string) (*image.Analysis, error) {
	// Heuristic: if imageName ends in .tar and source is docker, assume docker-archive
	if strings.HasSuffix(imageName, ".tar") && sourceStr == "docker" {
		sourceStr = "docker-archive"
		// If the file doesn't exist at the given path, check .data/
		if _, err := os.Stat(imageName); os.IsNotExist(err) {
			wd, _ := os.Getwd()
			// Navigate up from cmd/dive/cli/internal/mcp to root if needed
			// (During real runs, Getwd is project root)
			root := wd
			for i := 0; i < 5; i++ {
				if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
					break
				}
				root = filepath.Dir(root)
			}
			dataPath := filepath.Join(root, ".data", imageName)
			if _, err := os.Stat(dataPath); err == nil {
				imageName = dataPath
			}
		}
	}

	source := dive.ParseImageSource(sourceStr)
	if source == dive.SourceUnknown {
		return nil, fmt.Errorf("unknown image source: %s", sourceStr)
	}

	cacheKey := fmt.Sprintf("%s:%s", sourceStr, imageName)

	h.mu.RLock()
	analysis, ok := h.analyses[cacheKey]
	h.mu.RUnlock()

	if !ok {
		log.Infof("Image %s not in cache, analyzing...", imageName)
		resolver, err := dive.GetImageResolver(source)
		if err != nil {
			return nil, fmt.Errorf("cannot get image resolver: %v", err)
		}

		img, err := adapter.ImageResolver(resolver).Fetch(ctx, imageName)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch image: %v", err)
		}

		analysis, err = adapter.NewAnalyzer().Analyze(ctx, img)
		if err != nil {
			return nil, fmt.Errorf("cannot analyze image: %v", err)
		}

		h.mu.Lock()
		h.analyses[cacheKey] = analysis
		h.mu.Unlock()
	}
	return analysis, nil
}

func (h *toolHandlers) analyzeImageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	imageName, err := request.RequireString("image")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	sourceStr := request.GetString("source", "docker")

	analysis, err := h.getAnalysis(ctx, imageName, sourceStr)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	summary := h.formatSummary(analysis)
	return mcp.NewToolResultText(summary), nil
}

func (h *toolHandlers) formatSummary(analysis *image.Analysis) string {
	summary := fmt.Sprintf("Image: %s\n", analysis.Image)
	summary += fmt.Sprintf("Total Size: %d bytes\n", analysis.SizeBytes)
	summary += fmt.Sprintf("Efficiency Score: %.2f%%\n", analysis.Efficiency*100)
	summary += fmt.Sprintf("Wasted Space: %d bytes\n", analysis.WastedBytes)
	summary += fmt.Sprintf("Layers: %d\n", len(analysis.Layers))

	for i, layer := range analysis.Layers {
		summary += fmt.Sprintf("  Layer %d: %s (Size: %d bytes, Command: %s)\n", i, layer.ShortId(), layer.Size, layer.Command)
	}
	return summary
}

func (h *toolHandlers) getWastedSpaceHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	imageName, err := request.RequireString("image")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	sourceStr := request.GetString("source", "docker")

	analysis, err := h.getAnalysis(ctx, imageName, sourceStr)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if len(analysis.Inefficiencies) == 0 {
		return mcp.NewToolResultText("No wasted space detected in this image."), nil
	}

	summary := h.formatWastedSpace(analysis)
	return mcp.NewToolResultText(summary), nil
}

func (h *toolHandlers) formatWastedSpace(analysis *image.Analysis) string {
	summary := fmt.Sprintf("Top Inefficient Files for %s:\n", analysis.Image)
	limit := 20
	if len(analysis.Inefficiencies) < limit {
		limit = len(analysis.Inefficiencies)
	}

	for i := 0; i < limit; i++ {
		inef := analysis.Inefficiencies[i]
		summary += fmt.Sprintf("- %s (Cumulative Size: %d bytes, occurrences: %d)\n", inef.Path, inef.CumulativeSize, len(inef.Nodes))
	}
	return summary
}

func (h *toolHandlers) inspectLayerHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	imageName, err := request.RequireString("image")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	layerIdx, err := request.RequireInt("layer_index")
	if err != nil {
		return mcp.NewToolResultError("layer_index is required and must be an integer"), nil
	}

	sourceStr := request.GetString("source", "docker")
	pathStr := request.GetString("path", "/")

	analysis, err := h.getAnalysis(ctx, imageName, sourceStr)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if layerIdx < 0 || layerIdx >= len(analysis.RefTrees) {
		return mcp.NewToolResultError(fmt.Sprintf("layer index out of bounds: %d (total layers: %d)", layerIdx, len(analysis.RefTrees))), nil
	}

	tree := analysis.RefTrees[layerIdx]
	
	startNode := tree.Root
	if pathStr != "/" {
		node, err := tree.GetNode(pathStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("path not found in layer: %s", pathStr)), nil
		}
		startNode = node
	}

	summary := fmt.Sprintf("Contents of layer %d at %s:\n", layerIdx, pathStr)
	count := 0
	limit := 100

	for name, child := range startNode.Children {
		if count >= limit {
			summary += "... (truncated)\n"
			break
		}
		typeChar := "F"
		if child.Data.FileInfo.IsDir {
			typeChar = "D"
		}
		summary += fmt.Sprintf("[%s] %s (%d bytes)\n", typeChar, name, child.Data.FileInfo.Size)
		count++
	}

	if count == 0 {
		summary += "(Empty or no children found at this path)\n"
	}

	return mcp.NewToolResultText(summary), nil
}

// --- Resource Handlers ---

func (h *toolHandlers) resourceSummaryHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// URI pattern: dive://image/{name}/summary
	parts := strings.Split(request.Params.URI, "/")
	if len(parts) < 5 {
		return nil, fmt.Errorf("invalid resource URI: %s", request.Params.URI)
	}
	imageName := parts[3]
	
	analysis, err := h.getAnalysis(ctx, imageName, "docker")
	if err != nil {
		return nil, err
	}

	content := h.formatSummary(analysis)
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/plain",
			Text:     content,
		},
	}, nil
}

func (h *toolHandlers) resourceEfficiencyHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// URI pattern: dive://image/{name}/efficiency
	parts := strings.Split(request.Params.URI, "/")
	if len(parts) < 5 {
		return nil, fmt.Errorf("invalid resource URI: %s", request.Params.URI)
	}
	imageName := parts[3]
	
	analysis, err := h.getAnalysis(ctx, imageName, "docker")
	if err != nil {
		return nil, err
	}

	content := fmt.Sprintf("Efficiency Score: %.2f%%\nWasted Space: %d bytes\n", analysis.Efficiency*100, analysis.WastedBytes)
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/plain",
			Text:     content,
		},
	}, nil
}

// --- Prompt Handlers ---

func (h *toolHandlers) promptOptimizeDockerfileHandler(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	imageName, ok := request.Params.Arguments["image"]
	if !ok {
		return nil, fmt.Errorf("image argument is required")
	}

	analysis, err := h.getAnalysis(ctx, imageName, "docker")
	if err != nil {
		return nil, err
	}

	wasted := h.formatWastedSpace(analysis)
	summary := h.formatSummary(analysis)

	return &mcp.GetPromptResult{
		Description: "Optimize Dockerfile based on Dive analysis",
		Messages: []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("You are an expert in Docker and OCI image optimization. Your findings for image '%s':\n\n%s\n\n%s\n\nPlease suggest optimizations for the Dockerfile.", imageName, summary, wasted),
				},
			},
		},
	}, nil
}

func (h *toolHandlers) promptExplainLayerHandler(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	imageName, ok := request.Params.Arguments["image"]
	if !ok {
		return nil, fmt.Errorf("image argument is required")
	}
	layerIdxStr, ok := request.Params.Arguments["layer_index"]
	if !ok {
		return nil, fmt.Errorf("layer_index argument is required")
	}

	return &mcp.GetPromptResult{
		Description: "Explain the impact of a specific image layer",
		Messages: []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Can you explain what is happening in layer %s of image '%s'?", layerIdxStr, imageName),
				},
			},
		},
	}, nil
}
