package output

import (
	"encoding/json"
	"os"

	"github.com/JinkaiLiu/perf-loadgen/internal/config"
	"github.com/JinkaiLiu/perf-loadgen/pkg/types"
)

// JSONReport is the stable persisted report structure for the MVP.
type JSONReport struct {
	Config  jsonConfig    `json:"config"`
	Summary types.Summary `json:"summary"`
}

type jsonConfig struct {
	URL              string            `json:"url"`
	Method           string            `json:"method"`
	Headers          map[string]string `json:"headers,omitempty"`
	Body             string            `json:"body,omitempty"`
	BodyFile         string            `json:"body_file,omitempty"`
	PayloadDir       string            `json:"payload_dir,omitempty"`
	PayloadCount     int               `json:"payload_count,omitempty"`
	Streaming        bool              `json:"streaming,omitempty"`
	StreamFormat     string            `json:"stream_format,omitempty"`
	StreamDoneMarker string            `json:"stream_done_marker,omitempty"`
	StreamTextKeys   []string          `json:"stream_text_keys,omitempty"`
	StreamTokenKeys  []string          `json:"stream_token_keys,omitempty"`
	Concurrency      int               `json:"concurrency"`
	Duration         string            `json:"duration"`
	Requests         int64             `json:"requests,omitempty"`
	Timeout          string            `json:"timeout"`
	Output           string            `json:"output,omitempty"`
	QPS              float64           `json:"qps,omitempty"`
	RampUp           string            `json:"ramp_up,omitempty"`
	MetricsPort      int               `json:"metrics_port,omitempty"`
	Protocol         string            `json:"protocol,omitempty"`
	GRPCService      string            `json:"grpc_service,omitempty"`
	GRPCMethod       string            `json:"grpc_method,omitempty"`
	GRPCTLS          bool              `json:"grpc_tls,omitempty"`
	GRPCTokenField   string            `json:"grpc_token_field,omitempty"`
	WSSubprotocol    string            `json:"ws_subprotocol,omitempty"`
	ModelPricePer1K  float64           `json:"model_price_per_1k,omitempty"`
	AgentContext     string            `json:"agent_context,omitempty"`
}

// WriteJSONReport persists the config snapshot and summary to disk.
func WriteJSONReport(path string, cfg config.Config, summary types.Summary) error {
	report := JSONReport{
		Config: jsonConfig{
			URL:              cfg.URL,
			Method:           cfg.Method,
			Headers:          cfg.Headers,
			Body:             cfg.BodyText,
			BodyFile:         cfg.BodyFile,
			PayloadDir:       cfg.PayloadDir,
			PayloadCount:     cfg.PayloadCount,
			Streaming:        cfg.Streaming,
			StreamFormat:     cfg.StreamFormat,
			StreamDoneMarker: cfg.StreamDoneMarker,
			StreamTextKeys:   cfg.StreamTextKeys,
			StreamTokenKeys:  cfg.StreamTokenKeys,
			Concurrency:      cfg.Concurrency,
			Duration:         cfg.Duration.String(),
			Requests:         cfg.Requests,
			Timeout:          cfg.Timeout.String(),
			Output:           cfg.Output,
			QPS:              cfg.QPS,
			RampUp:           cfg.RampUp.String(),
			MetricsPort:      cfg.MetricsPort,
			Protocol:         cfg.Protocol,
			GRPCService:      cfg.GRPCService,
			GRPCMethod:       cfg.GRPCMethod,
			GRPCTLS:          cfg.GRPCTLS,
			GRPCTokenField:   cfg.GRPCTokenField,
			WSSubprotocol:    cfg.WSSubprotocol,
			ModelPricePer1K:  cfg.ModelPricePer1K,
			AgentContext:     cfg.AgentContext,
		},
		Summary: summary,
	}

	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}
