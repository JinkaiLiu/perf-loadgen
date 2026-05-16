package websocket

import (
	"github.com/JinkaiLiu/vibeready/internal/config"
	"github.com/JinkaiLiu/vibeready/internal/protocol/registry"
	"github.com/JinkaiLiu/vibeready/internal/runner"
)

func init() {
	registry.Register("websocket", func(cfg config.Config) (runner.Runner, error) {
		target := cfg.URL
		return NewRunner(target, cfg.WSSubprotocol, cfg.Timeout), nil
	})
}
