package config

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config is the validated runtime configuration for a load generation run.
type Config struct {
	URL              string            `json:"url"`
	Method           string            `json:"method"`
	Headers          map[string]string `json:"headers,omitempty"`
	Body             []byte            `json:"-"`
	BodyText         string            `json:"body,omitempty"`
	BodyFile         string            `json:"body_file,omitempty"`
	PayloadDir       string            `json:"payload_dir,omitempty"`
	PayloadCount     int               `json:"payload_count,omitempty"`
	PayloadFiles     []string          `json:"-"`
	Protocol         string            `json:"protocol,omitempty"`
	GRPCService      string            `json:"grpc_service,omitempty"`
	GRPCMethod       string            `json:"grpc_method,omitempty"`
	GRPCTLS          bool              `json:"grpc_tls,omitempty"`
	GRPCTokenField   string            `json:"grpc_token_field,omitempty"`
	WSSubprotocol    string            `json:"ws_subprotocol,omitempty"`
	ModelPricePer1K  float64           `json:"model_price_per_1k,omitempty"`
	Streaming        bool              `json:"streaming,omitempty"`
	StreamFormat     string            `json:"stream_format,omitempty"`
	StreamDoneMarker string            `json:"stream_done_marker,omitempty"`
	StreamTextKeys   []string          `json:"stream_text_keys,omitempty"`
	StreamTokenKeys  []string          `json:"stream_token_keys,omitempty"`
	Concurrency      int               `json:"concurrency"`
	Duration         time.Duration     `json:"duration"`
	Requests         int64             `json:"requests,omitempty"`
	Timeout          time.Duration     `json:"timeout"`
	Output           string            `json:"output,omitempty"`
	AgentContext     string            `json:"agent_context,omitempty"`
	QPS              float64           `json:"qps,omitempty"`
	RampUp           time.Duration     `json:"ramp_up,omitempty"`
	MetricsPort      int               `json:"metrics_port,omitempty"`

	Payloads [][]byte `json:"-"`
}

// Validate checks CLI config invariants and unsupported flags.
func (c Config) Validate() error {
	if strings.TrimSpace(c.URL) == "" {
		return errors.New("url is required")
	}
	// gRPC uses host:port format; HTTP/WebSocket require scheme://host.
	if c.Protocol == "grpc" || c.Protocol == "grpc-stream" {
		// Validate as host:port by appending a dummy scheme for parsing.
		parsed, err := url.Parse("grpc://" + c.URL)
		if err != nil || parsed.Host == "" {
			return fmt.Errorf("invalid gRPC target %q (expected host:port)", c.URL)
		}
	} else {
		parsed, err := url.ParseRequestURI(c.URL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("invalid url %q", c.URL)
		}
	}

	if c.Protocol == "" || c.Protocol == "http" {
		if strings.TrimSpace(c.Method) == "" {
			return errors.New("method is required")
		}
		if !validHTTPMethod(c.Method) {
			return fmt.Errorf("invalid method %q", c.Method)
		}
	}
	if c.Concurrency <= 0 {
		return errors.New("concurrency must be greater than 0")
	}
	if c.Duration < 0 {
		return errors.New("duration must be greater than or equal to 0")
	}
	if c.Requests < 0 {
		return errors.New("requests must be greater than or equal to 0")
	}
	if c.Duration == 0 && c.Requests == 0 {
		return errors.New("either duration or requests must be greater than 0")
	}
	if c.Timeout <= 0 {
		return errors.New("timeout must be greater than 0")
	}
	if c.StreamFormat == "" {
		c.StreamFormat = "auto"
	}
	if c.StreamDoneMarker == "" {
		c.StreamDoneMarker = "[DONE]"
	}
	if len(c.StreamTextKeys) == 0 {
		c.StreamTextKeys = []string{"content", "text", "token", "output_text", "delta"}
	}
	if len(c.StreamTokenKeys) == 0 {
		c.StreamTokenKeys = []string{"output_tokens", "completion_tokens", "generated_tokens"}
	}
	switch c.StreamFormat {
	case "auto", "sse", "jsonl", "raw":
	default:
		return fmt.Errorf("stream-format must be one of auto, sse, jsonl, raw")
	}
	switch c.Protocol {
	case "", "http", "grpc", "grpc-stream", "websocket":
	default:
		return fmt.Errorf("protocol must be one of http, grpc, grpc-stream, websocket")
	}
	if (c.Protocol == "grpc" || c.Protocol == "grpc-stream") && (c.GRPCService == "" || c.GRPCMethod == "") {
		return fmt.Errorf("--proto-service and --proto-method are required for gRPC protocol")
	}
	if c.QPS < 0 {
		return errors.New("qps must be greater than or equal to 0")
	}
	if c.RampUp < 0 {
		return errors.New("ramp-up must be greater than or equal to 0")
	}
	if c.RampUp > 0 && c.Duration > 0 && c.RampUp > c.Duration {
		return errors.New("ramp-up must not exceed duration")
	}
	if c.MetricsPort < 0 || c.MetricsPort > 65535 {
		return errors.New("metrics-port must be between 0 and 65535")
	}
	if len(c.Body) > 0 && strings.TrimSpace(c.BodyText) == "" {
		c.BodyText = string(c.Body)
	}
	if c.PayloadDir != "" && len(c.PayloadFiles) == 0 {
		return errors.New("payload-dir did not contain any readable payload files")
	}

	return nil
}

func validHTTPMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}
