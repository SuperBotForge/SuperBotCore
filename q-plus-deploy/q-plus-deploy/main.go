package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"q+/cmd/bot"
	"syscall"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Panic during app executing: %v\n", r)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	err := bot.Run(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error during app executing: %v\n", err)
	}
}
