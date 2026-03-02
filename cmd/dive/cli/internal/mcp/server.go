package mcp

import (
	"fmt"
	"net/http"

	"github.com/anchore/clio"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/wagoodman/dive/cmd/dive/cli/internal/options"
	"github.com/wagoodman/dive/internal/log"
)

func NewServer(id clio.Identification) *server.MCPServer {
	s := server.NewMCPServer(
		id.Name,
		id.Version,
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
		server.WithPromptCapabilities(true),
	)

	h := newToolHandlers()

	// --- Tools ---

	// 1. analyze_image tool
	analyzeTool := mcp.NewTool("analyze_image",
		mcp.WithDescription("Analyze a docker image and return efficiency metrics and layer details"),
		mcp.WithString("image",
			mcp.Required(),
			mcp.Description("The name of the image to analyze (e.g., 'ubuntu:latest')"),
		),
		mcp.WithString("source",
			mcp.Description("The container engine to fetch the image from (docker, podman, docker-archive). Defaults to 'docker'."),
		),
	)
	s.AddTool(analyzeTool, h.analyzeImageHandler)

	// 2. get_wasted_space tool
	wastedSpaceTool := mcp.NewTool("get_wasted_space",
		mcp.WithDescription("Get the list of inefficient files that contribute to wasted space in the image"),
		mcp.WithString("image",
			mcp.Required(),
			mcp.Description("The name of the image to get wasted space for (must be analyzed first)"),
		),
		mcp.WithString("source",
			mcp.Description("The container engine to fetch the image from (docker, podman, docker-archive). Defaults to 'docker'."),
		),
	)
	s.AddTool(wastedSpaceTool, h.getWastedSpaceHandler)

	// 3. inspect_layer tool
	inspectLayerTool := mcp.NewTool("inspect_layer",
		mcp.WithDescription("Inspect the contents of a specific layer in an image"),
		mcp.WithString("image",
			mcp.Required(),
			mcp.Description("The name of the image to inspect"),
		),
		mcp.WithNumber("layer_index",
			mcp.Required(),
			mcp.Description("The index of the layer to inspect (starting from 0)"),
		),
		mcp.WithString("source",
			mcp.Description("The container engine to fetch the image from (docker, podman, docker-archive). Defaults to 'docker'."),
		),
		mcp.WithString("path",
			mcp.Description("The path within the layer to inspect. Defaults to '/'."),
		),
	)
	s.AddTool(inspectLayerTool, h.inspectLayerHandler)

	// --- Resources ---

	// 1. Summary resource template
	summaryTemplate := mcp.NewResourceTemplate("dive://image/{name}/summary", "Image Summary",
		mcp.WithTemplateDescription("Get a text summary of the image analysis"),
	)
	s.AddResourceTemplate(summaryTemplate, h.resourceSummaryHandler)

	// 2. Efficiency resource template
	efficiencyTemplate := mcp.NewResourceTemplate("dive://image/{name}/efficiency", "Image Efficiency",
		mcp.WithTemplateDescription("Get the efficiency score and wasted bytes for an image"),
	)
	s.AddResourceTemplate(efficiencyTemplate, h.resourceEfficiencyHandler)

	// --- Prompts ---

	// 1. Optimize Dockerfile prompt
	s.AddPrompt(mcp.Prompt{
		Name:        "optimize-dockerfile",
		Description: "Get suggestions for optimizing a Dockerfile based on image analysis",
		Arguments: []mcp.PromptArgument{
			{
				Name:        "image",
				Description: "The name of the image to optimize",
				Required:    true,
			},
		},
	}, h.promptOptimizeDockerfileHandler)

	// 2. Explain Layer prompt
	s.AddPrompt(mcp.Prompt{
		Name:        "explain-layer",
		Description: "Get an explanation of the impact of a specific image layer",
		Arguments: []mcp.PromptArgument{
			{
				Name:        "image",
				Description: "The name of the image",
				Required:    true,
			},
			{
				Name:        "layer_index",
				Description: "The index of the layer to explain",
				Required:    true,
			},
		},
	}, h.promptExplainLayerHandler)

	return s
}

func Run(s *server.MCPServer, opts options.MCP) error {
	switch opts.Transport {
	case "sse":
		addr := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
		sseServer := server.NewSSEServer(s, server.WithBaseURL(fmt.Sprintf("http://%s", addr)))
		
		mux := http.NewServeMux()
		mux.Handle("/sse", sseServer.SSEHandler())
		mux.Handle("/messages", sseServer.MessageHandler())

		log.Infof("Starting MCP SSE server on %s", addr)
		fmt.Printf("Starting MCP SSE server on %s\n", addr)
		fmt.Printf("- SSE endpoint: http://%s/sse\n", addr)
		fmt.Printf("- Message endpoint: http://%s/messages\n", addr)

		return http.ListenAndServe(addr, mux)
	case "stdio":
		log.Infof("Starting MCP Stdio server")
		return server.ServeStdio(s)
	default:
		return fmt.Errorf("unsupported transport: %s", opts.Transport)
	}
}
