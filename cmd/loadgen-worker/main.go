package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/JinkaiLiu/vibeready/internal/cli"
	"github.com/JinkaiLiu/vibeready/internal/distributed"
)

func main() {
	opts, err := cli.ParseWorker(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse worker flags: %v\n", err)
		os.Exit(1)
	}

	worker := distributed.NewWorker(opts.ID, opts.ListenAddr, opts.Capacity, opts.AuthSecret)
	fmt.Printf("loadgen worker %s listening on %s (capacity=%d)\n", opts.ID, opts.ListenAddr, opts.Capacity)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		fmt.Println("\nshutting down worker...")
		_ = worker.Shutdown(context.Background())
	}()

	if err := worker.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "worker stopped: %v\n", err)
		os.Exit(1)
	}
}
