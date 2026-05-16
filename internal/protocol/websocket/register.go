package websocket

import (
	"github.com/JinkaiLiu/perf-loadgen/internal/config"
	"github.com/JinkaiLiu/perf-loadgen/internal/protocol/registry"
	"github.com/JinkaiLiu/perf-loadgen/internal/runner"
)

func init() {
	registry.Register("websocket", func(cfg config.Config) (runner.Runner, error) {
		target := cfg.URL
		return NewRunner(target, cfg.WSSubprotocol, cfg.Timeout), nil
	})
}
