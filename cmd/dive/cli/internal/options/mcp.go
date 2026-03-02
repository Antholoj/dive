package options

import (
	"github.com/anchore/clio"
)

var _ interface {
	clio.FlagAdder
} = (*MCP)(nil)

// MCP provides configuration for the Model Context Protocol server
type MCP struct {
	// Transport is the transport to use for the MCP server (stdio, sse)
	Transport string `yaml:"transport" json:"transport" mapstructure:"transport"`
	// Host is the host for the MCP HTTP/SSE server
	Host string `yaml:"host" json:"host" mapstructure:"host"`
	// Port is the port for the MCP HTTP/SSE server
	Port int `yaml:"port" json:"port" mapstructure:"port"`
}

func DefaultMCP() MCP {
	return MCP{
		Transport: "stdio",
		Host:      "localhost",
		Port:      8080,
	}
}

func (o *MCP) AddFlags(flags clio.FlagSet) {
	flags.StringVarP(&o.Transport, "transport", "t", "The transport to use for the MCP server (stdio, sse).")
	flags.StringVarP(&o.Host, "host", "", "The host to listen on for the MCP HTTP/SSE server.")
	flags.IntVarP(&o.Port, "port", "", "The port to listen on for the MCP HTTP/SSE server.")
}
