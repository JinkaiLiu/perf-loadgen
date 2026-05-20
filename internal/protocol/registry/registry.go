package registry

import (
	"fmt"
	"sync"

	"github.com/JinkaiLiu/vibeready/internal/config"
	"github.com/JinkaiLiu/vibeready/internal/runner"
)

// Factory constructs a protocol runner from validated config.
type Factory func(cfg config.Config) (runner.Runner, error)

var (
	mu        sync.RWMutex
	factories = map[string]Factory{}
)

// Register stores a runner factory for the given protocol name.
func Register(name string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	factories[name] = f
}

// Build creates a runner for the configured protocol.
// Defaults to "http" when cfg.Protocol is empty.
func Build(cfg config.Config) (runner.Runner, error) {
	name := cfg.Protocol
	if name == "" {
		name = "http"
	}

	mu.RLock()
	f, ok := factories[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown protocol %q", name)
	}
	return f(cfg)
}
