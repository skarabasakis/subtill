package core

import "testing"

func TestBootstrap(t *testing.T) {
	if Version == "" {
		t.Fatal("expected non-empty version")
	}
}
