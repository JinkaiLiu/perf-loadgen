package http

import (
	"github.com/JinkaiLiu/perf-loadgen/internal/config"
	"github.com/JinkaiLiu/perf-loadgen/internal/protocol/httpstream"
	"github.com/JinkaiLiu/perf-loadgen/internal/protocol/registry"
	"github.com/JinkaiLiu/perf-loadgen/internal/runner"
)

func init() {
	registry.Register("http", func(cfg config.Config) (runner.Runner, error) {
		if cfg.Streaming {
			return httpstream.NewRunner(httpstream.Options{
				Timeout:      cfg.Timeout,
				StreamFormat: cfg.StreamFormat,
				DoneMarker:   cfg.StreamDoneMarker,
				TextKeys:     cfg.StreamTextKeys,
				TokenKeys:    cfg.StreamTokenKeys,
			}), nil
		}
		return NewRunner(cfg.Timeout), nil
	})
}
