package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// fileConfig mirrors Config but uses strings for durations to support JSON unmarshaling.
type fileConfig struct {
	URL              string            `json:"url"`
	Method           string            `json:"method"`
	Headers          map[string]string `json:"headers,omitempty"`
	Body             string            `json:"body,omitempty"`
	BodyFile         string            `json:"body_file,omitempty"`
	PayloadDir       string            `json:"payload_dir,omitempty"`
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
	Duration         string            `json:"duration,omitempty"`
	Requests         int64             `json:"requests,omitempty"`
	Timeout          string            `json:"timeout,omitempty"`
	Output           string            `json:"output,omitempty"`
	AgentContext     string            `json:"agent_context,omitempty"`
	QPS              float64           `json:"qps,omitempty"`
	RampUp           string            `json:"ramp_up,omitempty"`
	MetricsPort      int               `json:"metrics_port,omitempty"`
}

func toConfig(fc fileConfig) (Config, error) {
	duration, err := time.ParseDuration(fc.Duration)
	if err != nil && fc.Duration != "" {
		return Config{}, fmt.Errorf("invalid duration %q: %w", fc.Duration, err)
	}
	timeout, err := time.ParseDuration(fc.Timeout)
	if err != nil && fc.Timeout != "" {
		return Config{}, fmt.Errorf("invalid timeout %q: %w", fc.Timeout, err)
	}
	rampUp, err := time.ParseDuration(fc.RampUp)
	if err != nil && fc.RampUp != "" {
		return Config{}, fmt.Errorf("invalid ramp-up %q: %w", fc.RampUp, err)
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	body := []byte(fc.Body)
	bodyText := fc.Body
	// body_file: read file content into Body (same as CLI behavior).
	if fc.BodyFile != "" && len(body) == 0 {
		data, err := os.ReadFile(fc.BodyFile)
		if err != nil {
			return Config{}, fmt.Errorf("read body_file %q: %w", fc.BodyFile, err)
		}
		body = data
		bodyText = string(data)
	}

	// payload_dir: list directory to populate PayloadFiles (same as CLI behavior).
	var payloadFiles []string
	var payloadCount int
	if fc.PayloadDir != "" {
		files, err := listPayloadDir(fc.PayloadDir)
		if err != nil {
			return Config{}, fmt.Errorf("read payload_dir %q: %w", fc.PayloadDir, err)
		}
		payloadFiles = files
		payloadCount = len(files)
	}

	return Config{
		URL:              fc.URL,
		Method:           fc.Method,
		Headers:          fc.Headers,
		Body:             body,
		BodyText:         bodyText,
		BodyFile:         fc.BodyFile,
		PayloadDir:       fc.PayloadDir,
		PayloadFiles:     payloadFiles,
		PayloadCount:     payloadCount,
		Protocol:         fc.Protocol,
		GRPCService:      fc.GRPCService,
		GRPCMethod:       fc.GRPCMethod,
		GRPCTLS:          fc.GRPCTLS,
		GRPCTokenField:   fc.GRPCTokenField,
		WSSubprotocol:    fc.WSSubprotocol,
		ModelPricePer1K:  fc.ModelPricePer1K,
		Streaming:        fc.Streaming,
		StreamFormat:     fc.StreamFormat,
		StreamDoneMarker: fc.StreamDoneMarker,
		StreamTextKeys:   fc.StreamTextKeys,
		StreamTokenKeys:  fc.StreamTokenKeys,
		Concurrency:      fc.Concurrency,
		Duration:         duration,
		Requests:         fc.Requests,
		Timeout:          timeout,
		Output:           fc.Output,
		AgentContext:     fc.AgentContext,
		QPS:              fc.QPS,
		RampUp:           rampUp,
		MetricsPort:      fc.MetricsPort,
	}, nil
}

func listPayloadDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(strings.TrimSpace(dir))
	if err != nil {
		return nil, fmt.Errorf("read payload dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	files := make([]string, len(names))
	for i, name := range names {
		files[i] = filepath.Join(dir, name)
	}
	return files, nil
}

// LoadFile reads and validates a JSON config file.
func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file: %w", err)
	}
	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return Config{}, fmt.Errorf("parse config file: %w", err)
	}
	cfg, err := toConfig(fc)
	if err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

// Merge overlays explicitly-set CLI values on top of file-based config.
func Merge(base, overlay Config, explicit map[string]bool) Config {
	apply := func(name string) bool { return explicit == nil || explicit[name] }

	if apply("url") {
		base.URL = overlay.URL
	}
	if apply("method") {
		base.Method = overlay.Method
	}
	if apply("headers") {
		base.Headers = overlay.Headers
	}
	if apply("body") || apply("body-file") {
		base.Body = overlay.Body
		base.BodyText = overlay.BodyText
		base.PayloadDir = ""
		base.PayloadFiles = nil
		base.PayloadCount = 0
	}
	if apply("body-file") {
		base.BodyFile = overlay.BodyFile
	} else if apply("body") {
		base.BodyFile = ""
	}
	if apply("payload-dir") {
		base.Body = nil
		base.BodyText = ""
		base.BodyFile = ""
		base.PayloadDir = overlay.PayloadDir
		base.PayloadFiles = overlay.PayloadFiles
		base.PayloadCount = overlay.PayloadCount
	}
	if apply("protocol") {
		base.Protocol = overlay.Protocol
	}
	if apply("proto-service") {
		base.GRPCService = overlay.GRPCService
	}
	if apply("proto-method") {
		base.GRPCMethod = overlay.GRPCMethod
	}
	if apply("grpc-tls") {
		base.GRPCTLS = overlay.GRPCTLS
	}
	if apply("grpc-token-field") {
		base.GRPCTokenField = overlay.GRPCTokenField
	}
	if apply("ws-subprotocol") {
		base.WSSubprotocol = overlay.WSSubprotocol
	}
	if apply("model-price") {
		base.ModelPricePer1K = overlay.ModelPricePer1K
	}
	if apply("stream") {
		base.Streaming = overlay.Streaming
	}
	if apply("stream-format") {
		base.StreamFormat = overlay.StreamFormat
	}
	if apply("stream-done-marker") {
		base.StreamDoneMarker = overlay.StreamDoneMarker
	}
	if apply("stream-text-keys") {
		base.StreamTextKeys = overlay.StreamTextKeys
	}
	if apply("stream-token-keys") {
		base.StreamTokenKeys = overlay.StreamTokenKeys
	}
	if apply("concurrency") {
		base.Concurrency = overlay.Concurrency
	}
	if apply("duration") {
		base.Duration = overlay.Duration
	}
	if apply("requests") {
		base.Requests = overlay.Requests
	}
	if apply("timeout") {
		base.Timeout = overlay.Timeout
	}
	if apply("output") {
		base.Output = overlay.Output
	}
	if apply("agent-context") {
		base.AgentContext = overlay.AgentContext
	}
	if apply("qps") {
		base.QPS = overlay.QPS
	}
	if apply("ramp-up") {
		base.RampUp = overlay.RampUp
	}
	if apply("metrics-port") {
		base.MetricsPort = overlay.MetricsPort
	}
	return base
}

// IsSet returns true if the config has enough fields set to be a valid load test.
func (c Config) IsSet() bool {
	return strings.TrimSpace(c.URL) != "" && strings.TrimSpace(c.Method) != ""
}
