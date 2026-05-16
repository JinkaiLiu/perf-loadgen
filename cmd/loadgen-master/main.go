package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JinkaiLiu/perf-loadgen/internal/cli"
	"github.com/JinkaiLiu/perf-loadgen/internal/distributed"
	"github.com/JinkaiLiu/perf-loadgen/internal/output"
)

func main() {
	cfg, opts, err := cli.ParseMaster(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse master flags: %v\n", err)
		os.Exit(1)
	}

	timeout := cfg.Timeout + cfg.Duration + 5*time.Second
	if timeout < 30*time.Second {
		timeout = 30 * time.Second
	}
	master := distributed.NewMaster(timeout, opts.AuthSecret, opts.MaxConcurrentJobs)

	if opts.DashboardAddr != "" {
		go func() {
			if err := master.ServeDashboard(opts.DashboardAddr); err != nil {
				fmt.Fprintf(os.Stderr, "dashboard stopped: %v\n", err)
			}
		}()
		fmt.Printf("dashboard listening on %s\n", opts.DashboardAddr)
	}

	if opts.Persistent {
		fmt.Println("master running in persistent mode")
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()
		master.StartBackground(ctx)
		<-ctx.Done()
		fmt.Println("\nshutting down...")
		master.Stop()
		return
	}

	if len(opts.Workers) == 0 {
		fmt.Fprintln(os.Stderr, "failed to parse master flags: --workers is required")
		os.Exit(1)
	}

	state, err := master.Run(context.Background(), cfg, opts.Workers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "distributed load test failed: %v\n", err)
		os.Exit(1)
	}

	if err := output.WriteConsoleSummary(os.Stdout, state.Summary); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write console summary: %v\n", err)
		os.Exit(1)
	}

	if cfg.Output != "" {
		if err := output.WriteJSONReport(cfg.Output, cfg, state.Summary); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write json output: %v\n", err)
			os.Exit(1)
		}
	}
	if cfg.AgentContext != "" {
		if err := output.WriteAgentReport(cfg.AgentContext, cfg, state.Summary); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write agent report: %v\n", err)
			os.Exit(1)
		}
	}
}
