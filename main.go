package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/skarabasakis/subtill/internal/cli"
	"github.com/skarabasakis/subtill/internal/desktop"
	httpx "github.com/skarabasakis/subtill/internal/http"
	"github.com/skarabasakis/subtill/internal/mcp"
	"github.com/skarabasakis/subtill/internal/runtime/console"
)

func main() {
	var (
		cmdMode    bool
		mcpMode    bool
		apiMode    bool
		apiPort    int
		apiToken   string
		configPath string
	)
	flag.BoolVar(&cmdMode, "cmd", false, "run CLI mode")
	flag.BoolVar(&mcpMode, "mcp", false, "run MCP server")
	flag.BoolVar(&apiMode, "api", false, "run REST API server")
	flag.IntVar(&apiPort, "port", 8000, "API listen port (--api)")
	flag.StringVar(&apiToken, "token", "", "API bearer token (--api)")
	flag.StringVar(&configPath, "config-path", "", "application config directory")
	flag.Parse()

	modes := 0
	if cmdMode {
		modes++
	}
	if mcpMode {
		modes++
	}
	if apiMode {
		modes++
	}
	if modes > 1 {
		fmt.Fprintln(os.Stderr, "only one of --cmd, --mcp, --api may be set")
		os.Exit(1)
	}

	if configPath != "" {
		_ = os.Setenv("SUBTILL_CONFIG_PATH", configPath)
	}

	if cmdMode || mcpMode || apiMode {
		if err := console.PrepareHeadless(); err != nil {
			fmt.Fprintf(os.Stderr, "console attach: %v\n", err)
		}
	}

	var err error
	switch {
	case cmdMode:
		err = cli.Run(flag.Args())
	case mcpMode:
		err = mcp.Run(context.Background())
	case apiMode:
		if apiToken == "" {
			fmt.Fprintln(os.Stderr, "--token is required with --api")
			os.Exit(1)
		}
		addr := fmt.Sprintf("127.0.0.1:%d", apiPort)
		err = httpx.ListenAndServe(httpx.ServerConfig{Addr: addr, Token: apiToken})
	default:
		err = desktop.Run()
	}
	if err != nil && !strings.Contains(err.Error(), "context canceled") {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
