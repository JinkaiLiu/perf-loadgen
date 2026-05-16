package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/JinkaiLiu/perf-loadgen/internal/protocol/httputil"
	"github.com/JinkaiLiu/perf-loadgen/pkg/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
)

// UnaryRunner executes single gRPC unary requests using dynamic method resolution.
type UnaryRunner struct {
	conn   *grpc.ClientConn
	opts   Options
	method *methodDescriptor
}

// NewUnaryRunner creates a gRPC unary runner with a shared connection.
func NewUnaryRunner(opts Options) (*UnaryRunner, error) {
	conn, err := dial(opts.Target, opts.TLS)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	md, err := resolveMethod(ctx, conn, opts.ServiceName, opts.MethodName)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("resolve method: %w", err)
	}

	return &UnaryRunner{
		conn:   conn,
		opts:   opts,
		method: md,
	}, nil
}

// Close shuts down the underlying gRPC connection.
func (r *UnaryRunner) Close() error {
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

// Run executes a single gRPC unary call with dynamic messages.
func (r *UnaryRunner) Run(ctx context.Context, req types.RequestSpec) (types.RunResult, error) {
	start := time.Now()

	input, err := buildInput(req.Body, r.method.Input)
	if err != nil {
		return httputil.FailedResult(types.ErrorCategoryUnknown, err, time.Since(start)), err
	}

	output := dynamicpb.NewMessage(r.method.Output)
	invokeErr := r.conn.Invoke(ctx, r.opts.FullMethod(), input, output)
	latency := time.Since(start)

	if invokeErr != nil {
		cat := classifyGRPCError(invokeErr)
		return httputil.FailedResult(cat, invokeErr, latency), invokeErr
	}

	bytesRead := int64(proto.Size(output))

	return types.RunResult{
		Success:       true,
		Latency:       latency,
		StatusCode:    0,
		ErrorCategory: types.ErrorCategoryNone,
		BytesRead:     bytesRead,
	}, nil
}

func classifyGRPCError(err error) types.ErrorCategory {
	st, ok := status.FromError(err)
	if !ok {
		return httputil.ClassifyRequestError(err)
	}
	switch st.Code() {
	case codes.DeadlineExceeded, codes.Canceled:
		return types.ErrorCategoryTimeout
	case codes.Unavailable, codes.Aborted, codes.ResourceExhausted:
		return types.ErrorCategoryNetwork
	default:
		return types.ErrorCategoryUnknown
	}
}
