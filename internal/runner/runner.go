package runner

import (
	"context"

	"github.com/JinkaiLiu/perf-loadgen/pkg/types"
)

// Runner executes a single request against a concrete protocol.
type Runner interface {
	Run(ctx context.Context, req types.RequestSpec) (types.RunResult, error)
}
