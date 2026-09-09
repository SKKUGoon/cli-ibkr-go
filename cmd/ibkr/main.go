package main

import (
	"context"
	"errors"
	"fmt"
	"ibkr-go/internal/cli"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := cli.NewRoot(os.Stdin, os.Stdout, os.Stderr).ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "입력을 취소했습니다.")
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
