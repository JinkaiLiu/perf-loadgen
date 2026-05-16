package protocol

import (
	"github.com/JinkaiLiu/perf-loadgen/internal/config"
	"github.com/JinkaiLiu/perf-loadgen/internal/protocol/registry"
	"github.com/JinkaiLiu/perf-loadgen/internal/runner"

	// Register protocol implementations.
	_ "github.com/JinkaiLiu/perf-loadgen/internal/protocol/grpc"
	_ "github.com/JinkaiLiu/perf-loadgen/internal/protocol/http"
	_ "github.com/JinkaiLiu/perf-loadgen/internal/protocol/httpstream"
	_ "github.com/JinkaiLiu/perf-loadgen/internal/protocol/websocket"
)

// BuildRunner selects the protocol runner for the current config.
func BuildRunner(cfg config.Config) (runner.Runner, error) {
	return registry.Build(cfg)
}
