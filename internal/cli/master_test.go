package cli

import "testing"

func TestParseMaster(t *testing.T) {
	t.Parallel()

	cfg, opts, err := ParseMaster([]string{
		"--workers", "http://127.0.0.1:8081,http://127.0.0.1:8082",
		"--dashboard-addr", ":7070",
		"--url", "http://localhost:8080/infer",
		"--method", "POST",
		"--body", "{}",
		"--concurrency", "2",
		"--duration", "1s",
	})
	if err != nil {
		t.Fatalf("ParseMaster returned error: %v", err)
	}
	if len(opts.Workers) != 2 {
		t.Fatalf("expected 2 workers, got %d", len(opts.Workers))
	}
	if opts.DashboardAddr != ":7070" {
		t.Fatalf("unexpected dashboard addr %q", opts.DashboardAddr)
	}
	if cfg.URL == "" || cfg.Method != "POST" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}
