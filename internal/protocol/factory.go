package protocol

import (
	"github.com/JinkaiLiu/vibeready/internal/config"
	"github.com/JinkaiLiu/vibeready/internal/protocol/registry"
	"github.com/JinkaiLiu/vibeready/internal/runner"

	// Register protocol implementations.
	_ "github.com/JinkaiLiu/vibeready/internal/protocol/grpc"
	_ "github.com/JinkaiLiu/vibeready/internal/protocol/http"
	_ "github.com/JinkaiLiu/vibeready/internal/protocol/httpstream"
	_ "github.com/JinkaiLiu/vibeready/internal/protocol/websocket"
)

// BuildRunner selects the protocol runner for the current config.
func BuildRunner(cfg config.Config) (runner.Runner, error) {
	return registry.Build(cfg)
}
