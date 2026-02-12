package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/bradgessler/agent-exec/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cmd.Execute(ctx)
}
