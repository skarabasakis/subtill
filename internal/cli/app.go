package cli

import (
	"fmt"
	"os"
)

// Run executes CLI mode (bootstrap stub).
func Run(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: subtill --cmd <command>")
		return fmt.Errorf("no command")
	}
	fmt.Printf("subtill cmd: %s\n", args[0])
	return nil
}
