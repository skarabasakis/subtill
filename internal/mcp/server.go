package mcp

import (
	"context"
	"fmt"
)

// Run starts the MCP server (bootstrap stub).
func Run(ctx context.Context) error {
	fmt.Println("subtill MCP server (bootstrap stub)")
	<-ctx.Done()
	return ctx.Err()
}
