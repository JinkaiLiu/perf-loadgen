package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/JinkaiLiu/vibeready/internal/config"
)

// MasterOptions holds distributed master specific flags.
type MasterOptions struct {
	Workers           []string
	DashboardAddr     string
	Persistent        bool
	AuthSecret        string
	MaxConcurrentJobs int
}

// ParseMaster parses master flags and the embedded job config.
func ParseMaster(args []string) (config.Config, MasterOptions, error) {
	jobArgs := make([]string, 0, len(args))
	var options MasterOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workers":
			if i+1 >= len(args) {
				return config.Config{}, MasterOptions{}, fmt.Errorf("--workers requires a value")
			}
			options.Workers = splitCSV(args[i+1])
			i++
		case "--dashboard-addr":
			if i+1 >= len(args) {
				return config.Config{}, MasterOptions{}, fmt.Errorf("--dashboard-addr requires a value")
			}
			options.DashboardAddr = strings.TrimSpace(args[i+1])
			i++
		case "--persistent":
			options.Persistent = true
		case "--auth-secret":
			if i+1 >= len(args) {
				return config.Config{}, MasterOptions{}, fmt.Errorf("--auth-secret requires a value")
			}
			options.AuthSecret = strings.TrimSpace(args[i+1])
			i++
		case "--max-concurrent-jobs":
			if i+1 >= len(args) {
				return config.Config{}, MasterOptions{}, fmt.Errorf("--max-concurrent-jobs requires a value")
			}
			fmt.Sscanf(args[i+1], "%d", &options.MaxConcurrentJobs)
			i++
		default:
			jobArgs = append(jobArgs, args[i])
		}
	}

	// In persistent mode, job config is optional — jobs come via API.
	if options.Persistent && len(jobArgs) == 0 {
		return config.Config{}, options, nil
	}
	cfg, err := Parse(jobArgs)
	if err != nil {
		return config.Config{}, MasterOptions{}, err
	}
	return cfg, options, nil
}

// WorkerOptions holds worker-specific flags.
type WorkerOptions struct {
	ID         string
	ListenAddr string
	Capacity   int
	AuthSecret string
}

// ParseWorker parses worker flags.
func ParseWorker(args []string) (WorkerOptions, error) {
	fs := flag.NewFlagSet("loadgen-worker", flag.ContinueOnError)
	idFlag := fs.String("id", "worker-1", "Worker identifier")
	listenFlag := fs.String("listen", ":8081", "Worker listen address")
	capacityFlag := fs.Int("capacity", 1, "Maximum concurrent jobs")
	authSecretFlag := fs.String("auth-secret", "", "Shared secret for HMAC authentication")
	if err := fs.Parse(args); err != nil {
		return WorkerOptions{}, err
	}

	return WorkerOptions{
		ID:         strings.TrimSpace(*idFlag),
		ListenAddr: strings.TrimSpace(*listenFlag),
		Capacity:   *capacityFlag,
		AuthSecret: strings.TrimSpace(*authSecretFlag),
	}, nil
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
