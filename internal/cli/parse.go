package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/JinkaiLiu/vibeready/internal/config"
)

// Parse converts CLI flags into validated runtime config.
func Parse(args []string) (config.Config, error) {
	// Extract --config before flag parsing so file is loaded first.
	var configPath string
	filteredArgs := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--config=") {
			configPath = strings.TrimPrefix(args[i], "--config=")
			continue
		}
		if args[i] == "--config" {
			if i+1 >= len(args) {
				return config.Config{}, errors.New("--config requires a file path")
			}
			configPath = args[i+1]
			i++ // skip the value
			continue
		}
		filteredArgs = append(filteredArgs, args[i])
	}

	var fileCfg config.Config
	if configPath != "" {
		loaded, err := config.LoadFile(configPath)
		if err != nil {
			return config.Config{}, err
		}
		fileCfg = loaded
	}

	fs := flag.NewFlagSet("loadgen", flag.ContinueOnError)

	var (
		urlFlag             = fs.String("url", "", "Target URL")
		methodFlag          = fs.String("method", "GET", "HTTP method")
		headersFlag         = fs.String("headers", "", "Comma-separated headers in key:value form")
		bodyFlag            = fs.String("body", "", "Inline request body")
		bodyFileFlag        = fs.String("body-file", "", "Read request body from file")
		payloadDirFlag      = fs.String("payload-dir", "", "Read and rotate payloads from a directory")
		protocolFlag        = fs.String("protocol", "http", "Protocol: http, grpc, grpc-stream")
		grpcServiceFlag     = fs.String("proto-service", "", "gRPC fully qualified service name")
		grpcMethodFlag      = fs.String("proto-method", "", "gRPC method name")
		grpcTLSFlag         = fs.Bool("grpc-tls", false, "Enable TLS for gRPC")
		grpcTokenFieldFlag  = fs.String("grpc-token-field", "", "gRPC response field name for token count")
		wsSubprotocolFlag   = fs.String("ws-subprotocol", "", "WebSocket subprotocol")
		modelPriceFlag      = fs.Float64("model-price", 0, "Model price per 1K output tokens for cost estimation")
		streamFlag          = fs.Bool("stream", false, "Enable streaming response processing")
		streamFormatFlag    = fs.String("stream-format", "auto", "Streaming format: auto, sse, jsonl, raw")
		streamDoneFlag      = fs.String("stream-done-marker", "[DONE]", "Streaming done marker for SSE/raw streams")
		streamTextKeysFlag  = fs.String("stream-text-keys", "", "Comma-separated preferred text keys for streamed JSON chunks")
		streamTokenKeysFlag = fs.String("stream-token-keys", "", "Comma-separated token count keys for streamed JSON chunks")
		concurrencyFlag     = fs.Int("concurrency", 1, "Number of concurrent workers")
		durationFlag        = fs.Duration("duration", 0, "Total run duration")
		requestsFlag        = fs.Int64("requests", 0, "Optional total request count")
		timeoutFlag         = fs.Duration("timeout", 30*time.Second, "Per-request timeout")
		outputFlag          = fs.String("output", "", "Optional JSON summary output path")
		agentContextFlag    = fs.String("agent-context", "", "Write agent-friendly markdown report to path")
		qpsFlag             = fs.Float64("qps", 0, "Optional global request rate limit")
		rampUpFlag          = fs.Duration("ramp-up", 0, "Optional linear worker ramp-up duration")
		metricsPortFlag     = fs.Int("metrics-port", 0, "Optional Prometheus metrics port")
	)

	if err := fs.Parse(filteredArgs); err != nil {
		return config.Config{}, err
	}

	// Track which flags were explicitly set (not using defaults).
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })

	sourceCount := 0
	for _, raw := range []string{*bodyFlag, *bodyFileFlag, *payloadDirFlag} {
		if strings.TrimSpace(raw) != "" {
			sourceCount++
		}
	}
	if sourceCount > 1 {
		return config.Config{}, errors.New("--body, --body-file, and --payload-dir are mutually exclusive")
	}

	headers, err := parseHeaders(*headersFlag)
	if err != nil {
		return config.Config{}, err
	}

	body, bodyText, payloadFiles, err := loadPayloadSource(*bodyFlag, *bodyFileFlag, *payloadDirFlag)
	if err != nil {
		return config.Config{}, err
	}

	cfg := config.Config{
		URL:              strings.TrimSpace(*urlFlag),
		Method:           strings.ToUpper(strings.TrimSpace(*methodFlag)),
		Headers:          headers,
		Body:             body,
		BodyText:         bodyText,
		BodyFile:         strings.TrimSpace(*bodyFileFlag),
		PayloadDir:       strings.TrimSpace(*payloadDirFlag),
		PayloadCount:     len(payloadFiles),
		PayloadFiles:     payloadFiles,
		Protocol:         strings.ToLower(strings.TrimSpace(*protocolFlag)),
		GRPCService:      strings.TrimSpace(*grpcServiceFlag),
		GRPCMethod:       strings.TrimSpace(*grpcMethodFlag),
		GRPCTLS:          *grpcTLSFlag,
		GRPCTokenField:   strings.TrimSpace(*grpcTokenFieldFlag),
		WSSubprotocol:    strings.TrimSpace(*wsSubprotocolFlag),
		ModelPricePer1K:  *modelPriceFlag,
		Streaming:        *streamFlag,
		StreamFormat:     strings.ToLower(strings.TrimSpace(*streamFormatFlag)),
		StreamDoneMarker: strings.TrimSpace(*streamDoneFlag),
		StreamTextKeys:   parseCSVList(*streamTextKeysFlag),
		StreamTokenKeys:  parseCSVList(*streamTokenKeysFlag),
		Concurrency:      *concurrencyFlag,
		Duration:         *durationFlag,
		Requests:         *requestsFlag,
		Timeout:          *timeoutFlag,
		Output:           strings.TrimSpace(*outputFlag),
		AgentContext:     strings.TrimSpace(*agentContextFlag),
		QPS:              *qpsFlag,
		RampUp:           *rampUpFlag,
		MetricsPort:      *metricsPortFlag,
	}

	// Merge: file config as base, only explicitly-set CLI flags override.
	if configPath != "" {
		cfg = config.Merge(fileCfg, cfg, explicit)
	}

	if err := cfg.Validate(); err != nil {
		return config.Config{}, err
	}

	return cfg, nil
}

func loadPayloadSource(body, bodyFile, payloadDir string) ([]byte, string, []string, error) {
	switch {
	case strings.TrimSpace(payloadDir) != "":
		files, err := listPayloadDir(payloadDir)
		if err != nil {
			return nil, "", nil, err
		}
		return nil, "", files, nil
	case strings.TrimSpace(bodyFile) != "":
		payload, err := os.ReadFile(strings.TrimSpace(bodyFile))
		if err != nil {
			return nil, "", nil, fmt.Errorf("read body file: %w", err)
		}
		return payload, string(payload), nil, nil
	default:
		return []byte(body), body, nil, nil
	}
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

func parseCSVList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, strings.ToLower(part))
		}
	}
	return out
}

func parseHeaders(raw string) (map[string]string, error) {
	headers := make(map[string]string)
	if strings.TrimSpace(raw) == "" {
		return headers, nil
	}

	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header %q, expected key:value", pair)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("invalid header %q, empty key", pair)
		}
		headers[key] = value
	}
	return headers, nil
}
