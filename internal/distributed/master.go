package distributed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/JinkaiLiu/vibeready/internal/config"
	"github.com/JinkaiLiu/vibeready/internal/stats"
	"github.com/JinkaiLiu/vibeready/pkg/types"
)

// Master coordinates distributed runs across workers.
type Master struct {
	client        *http.Client
	secret        string
	maxConcurrent int

	mu         sync.RWMutex
	latest     DashboardState
	queue      *JobQueue
	stopCh     chan struct{}
	sseClients map[chan []byte]struct{}
}

// NewMaster creates a master with a reusable HTTP client.
func NewMaster(timeout time.Duration, secret string, maxConcurrent int) *Master {
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}
	return &Master{
		client:        &http.Client{Timeout: timeout},
		secret:        secret,
		maxConcurrent: maxConcurrent,
		queue:         NewJobQueue(100),
		stopCh:        make(chan struct{}),
		sseClients:    make(map[chan []byte]struct{}),
	}
}

func (m *Master) broadcastState() {
	m.mu.RLock()
	data, _ := json.Marshal(m.latest)
	m.mu.RUnlock()

	msg := []byte(fmt.Sprintf("data: %s\n\n", data))
	m.mu.Lock()
	for ch := range m.sseClients {
		select {
		case ch <- msg:
		default:
			delete(m.sseClients, ch)
		}
	}
	m.mu.Unlock()
}

// StartBackground starts the background job processor for persistent mode.
func (m *Master) StartBackground(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stopCh:
				return
			default:
			}

			qj, err := m.queue.Dequeue(ctx)
			if err != nil {
				return
			}

			jobCtx, cancel := context.WithCancel(ctx)
			qj.Cancel = cancel

			state, err := m.Run(jobCtx, qj.Spec.Config, qj.Spec.Workers)
			if err != nil {
				m.queue.Fail(qj.ID, err)
			} else {
				m.queue.Complete(qj.ID, state)
			}
		}
	}()
}

// Stop gracefully shuts down the background processor.
func (m *Master) Stop() {
	close(m.stopCh)
}

// SubmitJob submits a job to the queue (persistent mode).
func (m *Master) SubmitJob(spec JobSpec) (string, error) {
	return m.queue.Enqueue(spec)
}

// GetJob returns job status by ID.
func (m *Master) GetJob(id string) (*JobInfo, bool) {
	return m.queue.Get(id)
}

// ListJobs returns all jobs.
func (m *Master) ListJobs() []*JobInfo {
	return m.queue.List()
}

// CancelJob cancels a running or pending job.
func (m *Master) CancelJob(id string) error {
	return m.queue.Cancel(id)
}

// Run executes a single job across the provided workers and returns the merged summary.
func (m *Master) Run(ctx context.Context, cfg config.Config, workers []string) (DashboardState, error) {
	if len(workers) == 0 {
		return DashboardState{}, fmt.Errorf("at least one worker is required")
	}
	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	started := time.Now()
	healthStates, err := m.CheckWorkers(ctx, workers)
	if err != nil {
		return DashboardState{}, err
	}
	m.setState(DashboardState{
		JobID:       jobID,
		Status:      "running",
		Started:     started,
		WorkerCount: len(workers),
		Workers:     healthStates,
	})

	collector := stats.NewCollector()
	if cfg.ModelPricePer1K > 0 {
		collector.SetPricing(cfg.ModelPricePer1K)
	}
	workerStates := make([]WorkerState, len(workers))
	copy(workerStates, healthStates)
	var wg sync.WaitGroup
	var stateMu sync.Mutex

	for i, workerAddr := range workers {
		workerCfg := shardConfig(cfg, i, len(workers))
		wg.Add(1)
		go func(index int, address string, jobConfig config.Config) {
			defer wg.Done()
			state := workerStates[index]
			state.Address = address
			state.Status = "running"
			result, err := m.dispatch(ctx, address, JobRequest{JobID: jobID, Config: jobConfig})
			stateMu.Lock()
			if err != nil {
				state.Status = "failed"
				state.Error = err.Error()
			} else {
				state.WorkerID = result.WorkerID
				state.Status = "completed"
				state.Result = result
				collector.MergeSnapshot(result.Snapshot)
			}
			workerStates[index] = state
			snapshot := m.snapshotState(jobID, started, workerStates, len(workers), collector.Summary())
			stateMu.Unlock()
			m.setState(snapshot)
		}(i, workerAddr, workerCfg)
	}

	wg.Wait()
	collector.SetWindow(started, time.Now())
	finalSummary := collector.Summary()
	completed := time.Now()
	finalStatus := "completed"
	for _, worker := range workerStates {
		if worker.Status == "failed" {
			finalStatus = "completed_with_errors"
			break
		}
	}

	finalState := DashboardState{
		JobID:       jobID,
		Status:      finalStatus,
		Started:     started,
		Completed:   completed,
		Summary:     finalSummary,
		Workers:     workerStates,
		WorkerCount: len(workers),
	}
	m.setState(finalState)
	return finalState, nil
}

// Handler exposes the master API and dashboard.
func (m *Master) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", m.handleDashboard)
	mux.HandleFunc("/api/latest", m.handleLatest)
	mux.HandleFunc("/api/jobs", m.handleJobs)
	mux.HandleFunc("/api/jobs/", m.handleJobByID)
	mux.HandleFunc("/api/workers", m.handleWorkers)
	mux.HandleFunc("/api/stream", m.handleSSE)
	mux.HandleFunc("/health", m.handleHealth)
	mux.HandleFunc("/ready", m.handleReady)
	return mux
}

func (m *Master) handleHealth(rw http.ResponseWriter, _ *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte(`{"status":"ok"}`))
}

func (m *Master) handleReady(rw http.ResponseWriter, _ *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte(`{"status":"ready"}`))
}

// CheckWorkers verifies health for each worker before dispatch.
func (m *Master) CheckWorkers(ctx context.Context, workers []string) ([]WorkerState, error) {
	states := make([]WorkerState, len(workers))
	for i, address := range workers {
		health, err := m.healthCheck(ctx, address)
		state := WorkerState{Address: address}
		if err != nil {
			state.Status = "unreachable"
			state.Error = err.Error()
			states[i] = state
			return states, fmt.Errorf("worker %s is not healthy: %w", address, err)
		}
		state.WorkerID = health.WorkerID
		if health.ActiveJobs >= health.Capacity && health.Capacity > 0 {
			state.Status = "busy"
			state.Health = health
			states[i] = state
			return states, fmt.Errorf("worker %s at capacity (%d/%d)", address, health.ActiveJobs, health.Capacity)
		}
		state.Status = health.Status
		state.Health = health
		states[i] = state
	}
	return states, nil
}

func (m *Master) dispatch(ctx context.Context, address string, job JobRequest) (WorkerResult, error) {
	payload, err := json.Marshal(job)
	if err != nil {
		return WorkerResult{}, err
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		url := strings.TrimRight(address, "/") + "/run"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return WorkerResult{}, err
		}
		req.Header.Set("Content-Type", "application/json")
		AddAuthHeader(req, job.JobID, m.secret)

		resp, err := m.client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 300 {
				lastErr = fmt.Errorf("worker %s returned %s: %s", address, resp.Status, strings.TrimSpace(string(body)))
			} else {
				var result WorkerResult
				if err := json.Unmarshal(body, &result); err != nil {
					lastErr = err
				} else if result.Error != "" {
					lastErr = fmt.Errorf("worker %s failed: %s", address, result.Error)
				} else {
					return result, nil
				}
			}
		}

		if ctx.Err() != nil {
			return WorkerResult{}, ctx.Err()
		}
		select {
		case <-ctx.Done():
			return WorkerResult{}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 150 * time.Millisecond):
		}
	}
	return WorkerResult{}, lastErr
}

func (m *Master) healthCheck(ctx context.Context, address string) (WorkerHealth, error) {
	url := strings.TrimRight(address, "/") + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return WorkerHealth{}, err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return WorkerHealth{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return WorkerHealth{}, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var health WorkerHealth
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return WorkerHealth{}, err
	}
	return health, nil
}

// CancelWorkerJob sends a cancel request to a worker for a specific job.
func (m *Master) CancelWorkerJob(ctx context.Context, address, jobID string) error {
	url := strings.TrimRight(address, "/") + "/cancel/" + jobID
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	AddAuthHeader(req, jobID, m.secret)
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func (m *Master) handleJobs(rw http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		var spec JobSpec
		if err := json.NewDecoder(req.Body).Decode(&spec); err != nil {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid job spec: " + err.Error()})
			return
		}
		if err := spec.Config.Validate(); err != nil {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if len(spec.Workers) == 0 {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "at least one worker is required"})
			return
		}
		id, err := m.SubmitJob(spec)
		if err != nil {
			writeJSON(rw, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(rw, http.StatusAccepted, map[string]string{"job_id": id, "status": "pending"})
	case http.MethodGet:
		writeJSON(rw, http.StatusOK, map[string]any{"jobs": m.ListJobs()})
	default:
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Master) handleJobByID(rw http.ResponseWriter, req *http.Request) {
	jobID := strings.TrimPrefix(req.URL.Path, "/api/jobs/")
	if jobID == "" {
		http.Error(rw, "missing job id", http.StatusBadRequest)
		return
	}
	switch req.Method {
	case http.MethodGet:
		info, ok := m.GetJob(jobID)
		if !ok {
			http.Error(rw, "job not found", http.StatusNotFound)
			return
		}
		writeJSON(rw, http.StatusOK, info)
	case http.MethodDelete:
		if err := m.CancelJob(jobID); err != nil {
			writeJSON(rw, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(rw, http.StatusOK, map[string]string{"status": "cancelled", "job_id": jobID})
	default:
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Master) handleSSE(rw http.ResponseWriter, req *http.Request) {
	flusher, ok := rw.(http.Flusher)
	if !ok {
		http.Error(rw, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 16)
	m.mu.Lock()
	m.sseClients[ch] = struct{}{}
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.sseClients, ch)
		m.mu.Unlock()
	}()

	// Send initial state.
	m.mu.RLock()
	data, _ := json.Marshal(m.latest)
	m.mu.RUnlock()
	_, _ = fmt.Fprintf(rw, "data: %s\n\n", data)
	flusher.Flush()

	for {
		select {
		case <-req.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, _ = rw.Write(msg)
			flusher.Flush()
		}
	}
}

func (m *Master) handleWorkers(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	workers := strings.TrimSpace(req.URL.Query().Get("addrs"))
	if workers == "" {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "?addrs=url1,url2 required"})
		return
	}
	addrs := strings.Split(workers, ",")
	states, _ := m.CheckWorkers(req.Context(), addrs)
	writeJSON(rw, http.StatusOK, map[string]any{"workers": states})
}

func shardConfig(cfg config.Config, index, total int) config.Config {
	if total <= 1 || cfg.Requests == 0 {
		return cfg
	}
	base := cfg.Requests / int64(total)
	remainder := cfg.Requests % int64(total)
	cfg.Requests = base
	if int64(index) < remainder {
		cfg.Requests++
	}
	return cfg
}

func (m *Master) handleLatest(rw http.ResponseWriter, _ *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	writeJSON(rw, http.StatusOK, m.latest)
}

func (m *Master) handleDashboard(rw http.ResponseWriter, _ *http.Request) {
	const page = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>vibeready dashboard</title>
  <style>
    :root { color-scheme: light; --bg:#f4f1ea; --panel:#fffdf8; --ink:#1e1b18; --muted:#6a625b; --accent:#b44536; --line:#ddd1c2; }
    body { margin:0; font-family: Georgia, "Iowan Old Style", serif; background: radial-gradient(circle at top, #fffdf8, #f0e6d7 60%, #e7dbc8); color:var(--ink); }
    .wrap { max-width: 1100px; margin: 0 auto; padding: 32px 20px 60px; }
    .hero { display:flex; justify-content:space-between; gap:20px; align-items:end; margin-bottom:24px; }
    .hero h1 { margin:0; font-size:42px; letter-spacing:-0.04em; }
    .hero p { margin:8px 0 0; color:var(--muted); }
    .grid { display:grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap:14px; margin-bottom:24px; }
    .card, .table { background:rgba(255,253,248,0.86); backdrop-filter: blur(8px); border:1px solid var(--line); border-radius:18px; box-shadow: 0 16px 40px rgba(92,66,40,0.08); }
    .card { padding:18px; }
    .label { font-size:12px; text-transform:uppercase; letter-spacing:0.12em; color:var(--muted); }
    .value { font-size:30px; margin-top:8px; }
    .table { padding:18px; }
    table { width:100%; border-collapse: collapse; }
    th, td { text-align:left; padding:10px 6px; border-bottom:1px solid var(--line); }
    th { color:var(--muted); font-size:12px; text-transform:uppercase; letter-spacing:0.08em; }
    .status { display:inline-block; padding:4px 10px; border-radius:999px; background:#f7d8d3; color:var(--accent); font-size:12px; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="hero">
      <div>
        <h1>vibeready distributed dashboard</h1>
        <p id="subtitle">Waiting for a job...</p>
      </div>
      <div class="status" id="job-status">idle</div>
    </div>
    <div class="grid" id="summary"></div>
    <div class="table">
      <table>
        <thead><tr><th>Worker</th><th>Address</th><th>Status</th><th>Total</th><th>QPS</th><th>TTFT</th><th>tok/s</th><th>Last Seen</th><th>Error</th></tr></thead>
        <tbody id="workers"></tbody>
      </table>
    </div>
  </div>
  <script>
    const fmt = (n) => typeof n === 'number' ? n.toFixed(2) : n;
    function render(data) {
      document.getElementById('job-status').textContent = data.status || 'idle';
      document.getElementById('subtitle').textContent = data.job_id ? ('Job ' + data.job_id + ' | workers ' + data.worker_count) : 'Waiting for a job...';
      const summary = data.summary || {};
      const cards = [
        ['Total', summary.total_requests || 0],
        ['Success', summary.successful_requests || 0],
        ['Failed', summary.failed_requests || 0],
        ['QPS', fmt(summary.requests_per_second || 0)],
        ['Avg TTFT', summary.avg_ttft_human || '0s'],
        ['Out Tokens', summary.total_output_tokens || 0],
        ['P95', summary.percentiles ? (summary.percentiles.p95_human || '0s') : '0s'],
        ['P99', summary.percentiles ? (summary.percentiles.p99_human || '0s') : '0s']
      ];
      document.getElementById('summary').innerHTML = cards.map(function(item) {
        return '<div class="card"><div class="label">' + item[0] + '</div><div class="value">' + item[1] + '</div></div>';
      }).join('');
      document.getElementById('workers').innerHTML = (data.workers || []).map((worker) => {
        const s = worker.result ? worker.result.summary : {};
        const health = worker.health || {};
        return '<tr><td>' + (worker.worker_id || '-') + '</td><td>' + (worker.address || '-') + '</td><td>' + (worker.status || '-') + '</td><td>' + (s.total_requests || 0) + '</td><td>' + fmt(s.requests_per_second || 0) + '</td><td>' + (s.avg_ttft_human || '0s') + '</td><td>' + fmt(s.avg_tokens_per_second || 0) + '</td><td>' + (health.last_seen || '-') + '</td><td>' + (worker.error || '') + '</td></tr>';
      }).join('');
    }
    if (window.EventSource) {
      const es = new EventSource('/api/stream');
      es.onmessage = function(e) { render(JSON.parse(e.data)); };
      es.onerror = function() { es.close(); fallbackPoll(); };
    } else {
      fallbackPoll();
    }
    function fallbackPoll() {
      setInterval(async function() {
        const res = await fetch('/api/latest');
        render(await res.json());
      }, 2000);
    }
    render({status: 'idle'});  </script>
</body>
</html>`

	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = rw.Write([]byte(page))
}

func (m *Master) setState(state DashboardState) {
	m.mu.Lock()
	m.latest = state
	m.mu.Unlock()
	m.broadcastState()
}

func (m *Master) snapshotState(jobID string, started time.Time, workers []WorkerState, workerCount int, summary types.Summary) DashboardState {
	copied := make([]WorkerState, len(workers))
	copy(copied, workers)
	return DashboardState{
		JobID:       jobID,
		Status:      "running",
		Started:     started,
		Summary:     summary,
		Workers:     copied,
		WorkerCount: workerCount,
	}
}

// ServeDashboard starts the master dashboard server.
func (m *Master) ServeDashboard(addr string) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           m.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server.ListenAndServe()
}

func writeJSON(rw http.ResponseWriter, status int, payload any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(payload)
}
