package httputil

import (
	"context"
	"errors"
	"net"
	"syscall"
	"time"

	"github.com/JinkaiLiu/vibeready/pkg/types"
)

// ClassifyRequestError maps a Go error to a stable error category.
func ClassifyRequestError(err error) types.ErrorCategory {
	if err == nil {
		return types.ErrorCategoryNone
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return types.ErrorCategoryTimeout
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return types.ErrorCategoryTimeout
		}
		return types.ErrorCategoryNetwork
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		return types.ErrorCategoryNetwork
	}

	return types.ErrorCategoryUnknown
}

// FailedResult builds a failed RunResult with the given category and error.
func FailedResult(category types.ErrorCategory, err error, latency time.Duration) types.RunResult {
	return types.RunResult{
		Success:       false,
		Latency:       latency,
		ErrorCategory: category,
		ErrorMessage:  err.Error(),
	}
}
