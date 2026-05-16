package grpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFullMethod(t *testing.T) {
	o := Options{ServiceName: "echo.Echo", MethodName: "Unary"}
	if got := o.FullMethod(); got != "/echo.Echo/Unary" {
		t.Fatalf("FullMethod = %q, want /echo.Echo/Unary", got)
	}
}

func TestFullMethodEmpty(t *testing.T) {
	o := Options{}
	if got := o.FullMethod(); got != "" {
		t.Fatalf("expected empty FullMethod, got %q", got)
	}
}

func TestClassifyGRPCError(t *testing.T) {
	tests := []struct {
		code codes.Code
		want string
	}{
		{codes.DeadlineExceeded, "timeout"},
		{codes.Canceled, "timeout"},
		{codes.Unavailable, "network"},
		{codes.Aborted, "network"},
		{codes.ResourceExhausted, "network"},
		{codes.InvalidArgument, "unknown"},
		{codes.Internal, "unknown"},
	}
	for _, tc := range tests {
		err := status.Error(tc.code, "test")
		cat := classifyGRPCError(err)
		if string(cat) != tc.want {
			t.Errorf("classifyGRPCError(%s) = %s, want %s", tc.code, cat, tc.want)
		}
	}
}

func TestClassifyGRPCErrorNonGRPC(t *testing.T) {
	cat := classifyGRPCError(context.DeadlineExceeded)
	if cat != "timeout" {
		t.Errorf("expected timeout for context.DeadlineExceeded, got %s", cat)
	}
}

func TestExtractTokenCount(t *testing.T) {
	data, _ := json.Marshal(map[string]any{
		"output_tokens": float64(42),
		"text":          "hello world",
	})
	if got := extractTokenCount(data, "output_tokens"); got != 42 {
		t.Fatalf("extractTokenCount = %d, want 42", got)
	}
}

func TestExtractTokenCountNested(t *testing.T) {
	data, _ := json.Marshal(map[string]any{
		"usage": map[string]any{
			"completion_tokens": float64(100),
		},
	})
	if got := extractTokenCount(data, "completion_tokens"); got != 100 {
		t.Fatalf("extractTokenCount = %d, want 100", got)
	}
}

func TestExtractTokenCountEmpty(t *testing.T) {
	if got := extractTokenCount(nil, "foo"); got != 0 {
		t.Fatalf("expected 0 for nil data, got %d", got)
	}
	if got := extractTokenCount([]byte(`{}`), ""); got != 0 {
		t.Fatalf("expected 0 for empty field, got %d", got)
	}
}

func TestFindInt(t *testing.T) {
	data := map[string]any{
		"level1": map[string]any{
			"level2": float64(99),
		},
	}
	if got := findInt(data, "level2"); got != 99 {
		t.Fatalf("findInt = %d, want 99", got)
	}
}

func TestOptions(t *testing.T) {
	opts := Options{
		Target:      "localhost:50051",
		Timeout:     5 * time.Second,
		TLS:         false,
		ServiceName: "test.Service",
		MethodName:  "Call",
		TokenField:  "tokens",
	}
	if opts.Target != "localhost:50051" {
		t.Errorf("unexpected target")
	}
	if opts.FullMethod() != "/test.Service/Call" {
		t.Errorf("unexpected full method: %s", opts.FullMethod())
	}
}
