# Dive MCP Server: Comprehensive Guide

This guide provides detailed instructions on how to build and run the `dive` Model Context Protocol (MCP) server and connect various clients to it.

---

## 1. Building the Server

Before running the MCP server, you need to build the `dive` binary from source to ensure you have the latest MCP features.

### Prerequisites
- **Go**: Version 1.24 or higher.
- **Task** (Optional): A task runner for simplified builds.

### Standard Build (Recommended)
If you have `task` installed:
```bash
# Build the binary for your local platform
task build-local
```
The binary will be created at `./dive`.

### Manual Build
Alternatively, use the standard Go compiler:
```bash
go build -o dive ./cmd/dive
```

---

## 2. Running the Dive MCP Server

The `dive` binary includes a built-in MCP server. You can run it using the `mcp` subcommand.

### Transport Options

Dive supports three transport modes for MCP:

#### A. Stdio (Standard Input/Output)
Best for local AI agents running on the same machine (e.g., Claude Desktop, Cursor). This is the default transport.
```bash
# Run with default settings
dive mcp

# Recommended for production (suppresses non-protocol logs)
dive mcp --quiet
```

#### B. Streamable HTTP (Unified HTTP + SSE) - *Recommended for Network*
The modern standard for MCP communication over HTTP. Consolidates handshakes and messages into a single endpoint.
```bash
dive mcp --transport streamable-http --port 8080
```
- **Endpoint:** `http://localhost:8080/mcp`

#### C. SSE (Server-Sent Events) - *Legacy Support*
Maintained for backwards compatibility with older MCP clients. In Dive, this is internally routed through the Streamable HTTP engine for maximum robustness.
```bash
dive mcp --transport sse --port 8080
```
- **SSE Endpoint:** `http://localhost:8080/sse`
- **Message Endpoint:** `http://localhost:8080/message`

---

## 3. Connecting MCP Clients

### Claude Desktop
Add `dive` to your `claude_desktop_config.json` (usually found in `%APPDATA%\Claude\claude_desktop_config.json` on Windows or `~/Library/Application Support/Claude/claude_desktop_config.json` on macOS).

**Using Stdio:**
```json
{
  "mcpServers": {
    "dive": {
      "command": "/usr/local/bin/dive",
      "args": ["mcp", "--quiet"]
    }
  }
}
```

**Using HTTP (Streamable):**
```json
{
  "mcpServers": {
    "dive": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

### Gemini-CLI
Gemini-CLI can connect to Dive via HTTP or Stdio.

**Configuration (`~/.gemini-cli/config.yaml`):**
```yaml
mcpServers:
  dive:
    url: "http://localhost:8080/mcp"
```

### IDEs (Cursor, VS Code via Roo Code)
1. Open the MCP settings in your IDE.
2. Add a new MCP server.
3. Choose **command** mode and use:
   - Command: `dive`
   - Arguments: `mcp`, `--quiet`

---

## 4. Available Tools & Capabilities

Once connected, your AI agent will have access to the following tools:

| Tool | Purpose |
| :--- | :--- |
| `analyze_image` | Get high-level efficiency metrics and layer metadata for any image. |
| `get_wasted_space` | Identify specific files that are duplicated or deleted across layers. |
| `inspect_layer` | Explore the file tree of a specific layer at a specific path. |
| `diff_layers` | Compare any two layers to see added, modified, or removed files. |

### Resource Templates
Agents can also "read" analysis results via URI:
- `dive://image/{name}/summary`
- `dive://image/{name}/efficiency`

---

## 5. Powerful Usage Examples

Once the server is connected, you can use natural language prompts to perform complex analysis. Here are a few examples of what you can ask your AI agent:

### A. Efficiency Health Check
> "Analyze the image 'my-app:latest' and tell me the efficiency score and total wasted space. Is it performing well compared to industry standards?"

### B. Identifying Bloat
> "Show me the top 10 most inefficient files in 'node:20-alpine'. I want to see which layers they are located in and how much space they are wasting."

### C. Troubleshooting Layer Growth
> "I just added a new layer to my Dockerfile and the image size jumped by 500MB unexpectedly. Can you inspect the latest layer of 'my-app:dev' and find the specific files causing this growth?"

### D. Deep Diffing for Debugging
> "Compare layer 3 and layer 4 of 'custom-service:v2'. Show me exactly which files were added or modified in that transition so I can understand the build impact."

### E. AI-Driven Optimization
> "Use the 'optimize-dockerfile' prompt for image 'python:3.11-slim'. Based on the analysis, give me 3 specific recommendations to reduce the image size without breaking the application."

---

## 6. Security & Performance Tuning

### Security Sandbox
To prevent the AI from accessing arbitrary tarballs on your host, use the sandbox flag to restrict `docker-archive` lookups:
```bash
dive mcp --mcp-sandbox /home/user/allowed-images/
```

### Analysis Caching
Image analysis is computationally expensive. Dive maintains an LRU cache to speed up repeated requests:
- `--mcp-cache-size`: Number of analysis results to keep (default: 10).
- `--mcp-cache-ttl`: How long to keep results (e.g., `24h`, `1h30m`).

### Persistent Settings
Save your preferred MCP configuration in `~/.dive.yaml`:
```yaml
mcp:
  transport: streamable-http
  port: 8080
  mcp-sandbox: /tmp/images
  mcp-cache-size: 20
```

---

## 7. Troubleshooting: "Missing sessionId"
If you encounter a `Missing sessionId` error when using SSE, ensure you are providing the `Mcp-Session-Id` header in your HTTP requests. 

**Recommendation:** Switch to the `streamable-http` transport and use the `/mcp` endpoint, which handles session negotiation automatically during the initial handshake.
