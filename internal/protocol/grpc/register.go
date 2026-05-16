package grpc

import (
	"fmt"

	"github.com/JinkaiLiu/perf-loadgen/internal/config"
	"github.com/JinkaiLiu/perf-loadgen/internal/protocol/registry"
	"github.com/JinkaiLiu/perf-loadgen/internal/runner"
)

func init() {
	registry.Register("grpc", func(cfg config.Config) (runner.Runner, error) {
		if cfg.GRPCService == "" || cfg.GRPCMethod == "" {
			return nil, fmt.Errorf("--proto-service and --proto-method are required for gRPC")
		}
		return NewUnaryRunner(Options{
			Target:      cfg.URL,
			Timeout:     cfg.Timeout,
			TLS:         cfg.GRPCTLS,
			ServiceName: cfg.GRPCService,
			MethodName:  cfg.GRPCMethod,
		})
	})

	registry.Register("grpc-stream", func(cfg config.Config) (runner.Runner, error) {
		if cfg.GRPCService == "" || cfg.GRPCMethod == "" {
			return nil, fmt.Errorf("--proto-service and --proto-method are required for gRPC streaming")
		}
		return NewStreamRunner(Options{
			Target:      cfg.URL,
			Timeout:     cfg.Timeout,
			TLS:         cfg.GRPCTLS,
			ServiceName: cfg.GRPCService,
			MethodName:  cfg.GRPCMethod,
			TokenField:  cfg.GRPCTokenField,
		})
	})
}
