package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/JinkaiLiu/perf-loadgen/internal/cli"
	"github.com/JinkaiLiu/perf-loadgen/internal/engine"
	"github.com/JinkaiLiu/perf-loadgen/internal/output"
	"github.com/JinkaiLiu/perf-loadgen/internal/protocol"
)

func main() {
	cfg, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse flags: %v\n", err)
		os.Exit(1)
	}

	runner, err := protocol.BuildRunner(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build runner: %v\n", err)
		os.Exit(1)
	}
	observers := make([]engine.Observer, 0, 1)
	var exporter *output.PrometheusExporter
	if cfg.MetricsPort > 0 {
		exporter = output.NewPrometheusExporter(cfg.MetricsPort)
		exporter.Start()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = exporter.Shutdown(shutdownCtx)
		}()
		observers = append(observers, exporter)
	}

	summary, err := engine.New(runner, observers...).Run(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load test failed: %v\n", err)
		os.Exit(1)
	}

	if err := output.WriteConsoleSummary(os.Stdout, summary); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write console summary: %v\n", err)
		os.Exit(1)
	}

	if cfg.Output != "" {
		if err := output.WriteJSONReport(cfg.Output, cfg, summary); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write json output: %v\n", err)
			os.Exit(1)
		}
	}
	if cfg.AgentContext != "" {
		if err := output.WriteAgentReport(cfg.AgentContext, cfg, summary); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write agent report: %v\n", err)
			os.Exit(1)
		}
	}
}
