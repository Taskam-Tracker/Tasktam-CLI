// Command taskam is the Taskam command-line client.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/Taskam-Tracker/Tasktam-CLI/internal/cmd"
)

// version is set at build time: -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cmd.Execute(ctx, version)
	stop()
	os.Exit(code)
}
