package mcp

import (
	"context"
	"fmt"
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
		return mcp.NewToolResultText("No wasted space detected in this image.")
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
