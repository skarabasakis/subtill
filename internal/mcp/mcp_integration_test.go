//go:build integration

package mcp_test

import (
	"context"
	"testing"
	"time"

	"github.com/skarabasakis/subtill/internal/mcp"
)

func TestMCPStubRuns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := mcp.Run(ctx)
	if err == nil {
		t.Fatal("expected context deadline error")
	}
}
