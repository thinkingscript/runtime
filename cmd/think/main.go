package main

import (
	"context"
)

func main() {
	// No SIGINT interception. Ctrl+C outside of raw-mode prompts
	// generates SIGINT and the OS kills us immediately.
	// During raw-mode prompts (huh), Ctrl+C is handled as a keypress
	// and the prompt calls os.Exit(130) directly.
	execute(context.Background())
}
