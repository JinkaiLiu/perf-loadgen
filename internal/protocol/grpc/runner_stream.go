package grpc

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/JinkaiLiu/perf-loadgen/internal/protocol/httputil"
	"github.com/JinkaiLiu/perf-loadgen/pkg/types"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
)

// StreamRunner executes gRPC server-streaming requests using dynamic method resolution.
type StreamRunner struct {
	conn       *grpc.ClientConn
	opts       Options
	method     *methodDescriptor
	tokenField string
}

var streamDesc = &grpc.StreamDesc{
	StreamName:    "ServerStream",
	ServerStreams: true,
}

// NewStreamRunner creates a gRPC server-streaming runner.
func NewStreamRunner(opts Options) (*StreamRunner, error) {
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

	return &StreamRunner{
		conn:       conn,
		opts:       opts,
		method:     md,
		tokenField: opts.TokenField,
	}, nil
}

// Close shuts down the underlying gRPC connection.
func (r *StreamRunner) Close() error {
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

// Run executes a single gRPC server-streaming call and extracts streaming metrics.
func (r *StreamRunner) Run(ctx context.Context, req types.RequestSpec) (types.RunResult, error) {
	start := time.Now()

	input, err := buildInput(req.Body, r.method.Input)
	if err != nil {
		return httputil.FailedResult(types.ErrorCategoryUnknown, err, time.Since(start)), err
	}

	stream, err := r.conn.NewStream(ctx, streamDesc, r.opts.FullMethod())
	if err != nil {
		cat := classifyGRPCError(err)
		return httputil.FailedResult(cat, err, time.Since(start)), err
	}

	if err := stream.SendMsg(input); err != nil {
		cat := classifyGRPCError(err)
		result := httputil.FailedResult(cat, err, time.Since(start))
		result.StreamingAborted = true
		return result, err
	}
	if err := stream.CloseSend(); err != nil {
		cat := classifyGRPCError(err)
		return httputil.FailedResult(cat, err, time.Since(start)), err
	}

	result := types.RunResult{
		Success:       true,
		ErrorCategory: types.ErrorCategoryNone,
	}

	var (
		bytesRead      int64
		firstChunkTime time.Time
		lastChunkTime  time.Time
		prevChunkTime  time.Time
		totalTokens    int64
		chunkCount     int
		itlSamples     []time.Duration
	)

	for {
		chunk := dynamicpb.NewMessage(r.method.Output)
		err := stream.RecvMsg(chunk)
		if err == io.EOF {
			break
		}
		if err != nil {
			cat := classifyGRPCError(err)
			result.Success = false
			result.ErrorCategory = cat
			result.ErrorMessage = err.Error()
			result.BytesRead = bytesRead
			result.Latency = time.Since(start)
			result.StreamingAborted = true
			return result, err
		}

		chunkCount++
		chunkSize := int64(proto.Size(chunk))
		bytesRead += chunkSize
		now := time.Now()
		if firstChunkTime.IsZero() {
			firstChunkTime = now
		} else {
			itlSamples = append(itlSamples, now.Sub(prevChunkTime))
		}
		prevChunkTime = now
		lastChunkTime = now

		if jsonData := toJSON(chunk); len(jsonData) > 0 {
			totalTokens += extractTokenCount(jsonData, r.tokenField)
		}
	}

	result.BytesRead = bytesRead
	result.Latency = time.Since(start)
	result.ITLSamples = itlSamples
	if !firstChunkTime.IsZero() {
		result.TTFT = firstChunkTime.Sub(start)
	}
	if !lastChunkTime.IsZero() && !firstChunkTime.IsZero() {
		result.GenerationTime = lastChunkTime.Sub(firstChunkTime)
	}
	result.OutputTokens = totalTokens
	if result.OutputTokens > 0 && result.GenerationTime > 0 {
		result.TokensPerSecond = float64(result.OutputTokens) / result.GenerationTime.Seconds()
	}
	if result.OutputTokens == 0 && chunkCount > 0 {
		result.OutputTokens = int64(chunkCount)
	}

	return result, nil
}
