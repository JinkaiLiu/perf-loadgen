package distributed

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/JinkaiLiu/vibeready/internal/engine"
	"github.com/JinkaiLiu/vibeready/internal/protocol"
)

// jobHandle tracks an in-flight job on the worker.
type jobHandle struct {
	id     string
	cancel context.CancelFunc
}

// Worker hosts a local execution endpoint for distributed jobs.
type Worker struct {
	id         string
	listenAddr string
	capacity   int
	secret     string

	mu         sync.RWMutex
	jobs       map[string]*jobHandle
	lastJobID  string
	lastSeen   time.Time
}

// NewWorker creates a worker with the given configuration.
func NewWorker(id, listenAddr string, capacity int, secret string) *Worker {
	if capacity <= 0 {
		capacity = 1
	}
	return &Worker{
		id:         id,
		listenAddr: listenAddr,
		capacity:   capacity,
		secret:     secret,
		jobs:       make(map[string]*jobHandle),
		lastSeen:   time.Now(),
	}
}

// Handler returns the worker HTTP handler.
func (w *Worker) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", w.handleHealth)
	mux.HandleFunc("/run", w.handleRun)
	mux.HandleFunc("/cancel/", w.handleCancel)
	mux.HandleFunc("/status/", w.handleStatus)

	var h http.Handler = mux
	if w.secret != "" {
		h = AuthMiddleware(w.secret, mux)
	}
	return h
}

// ListenAndServe starts the worker server.
func (w *Worker) ListenAndServe() error {
	server := &http.Server{
		Addr:              w.listenAddr,
		Handler:           w.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server.ListenAndServe()
}

func (w *Worker) handleHealth(rw http.ResponseWriter, _ *http.Request) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	jobIDs := make([]string, 0, len(w.jobs))
	for id := range w.jobs {
		jobIDs = append(jobIDs, id)
	}

	currentJobID := ""
	if len(jobIDs) > 0 {
		currentJobID = jobIDs[0]
	}
	writeJSON(rw, http.StatusOK, WorkerHealth{
		WorkerID:     w.id,
		Status:       workerStatus(len(w.jobs), w.capacity),
		Busy:         len(w.jobs) >= w.capacity,
		CurrentJobID: currentJobID,
		LastJobID:    w.lastJobID,
		LastSeen:     w.lastSeen,
		ListenAddr:   w.listenAddr,
		Capacity:     w.capacity,
		ActiveJobs:   len(w.jobs),
		JobIDs:       jobIDs,
	})
}

func (w *Worker) handleRun(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var job JobRequest
	if err := json.NewDecoder(http.MaxBytesReader(rw, req.Body, 1<<20)).Decode(&job); err != nil {
		http.Error(rw, "invalid job payload", http.StatusBadRequest)
		return
	}
	if err := job.Config.Validate(); err != nil {
		http.Error(rw, "invalid job config: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Verify HMAC auth for /run.
	if !VerifyJob(req, job.JobID, w.secret) {
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.mu.Lock()
	if len(w.jobs) >= w.capacity {
		activeIDs := make([]string, 0, len(w.jobs))
		for id := range w.jobs {
			activeIDs = append(activeIDs, id)
		}
		w.mu.Unlock()
		http.Error(rw, "worker at capacity, active jobs: "+strings.Join(activeIDs, ","), http.StatusConflict)
		return
	}
	jobCtx, cancel := context.WithCancel(req.Context())
	w.jobs[job.JobID] = &jobHandle{id: job.JobID, cancel: cancel}
	w.mu.Unlock()

	started := time.Now()
	runner, buildErr := protocol.BuildRunner(job.Config)
	if buildErr != nil {
		// Report the error as a worker result rather than crashing.
		w.mu.Lock()
		delete(w.jobs, job.JobID)
		w.lastSeen = time.Now()
		w.mu.Unlock()
		writeJSON(rw, http.StatusBadRequest, WorkerResult{
			WorkerID: w.id,
			JobID:    job.JobID,
			Error:    "failed to build runner: " + buildErr.Error(),
		})
		return
	}
	report, err := engine.New(runner).RunDetailed(jobCtx, job.Config)
	ended := time.Now()

	w.mu.Lock()
	delete(w.jobs, job.JobID)
	w.lastJobID = job.JobID
	w.lastSeen = ended
	w.mu.Unlock()

	result := WorkerResult{
		WorkerID: w.id,
		JobID:    job.JobID,
		Started:  started,
		Ended:    ended,
		Snapshot: report.Snapshot,
		Summary:  report.Summary,
	}
	if err != nil {
		result.Error = err.Error()
	}

	writeJSON(rw, http.StatusOK, result)
}

func (w *Worker) handleCancel(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := strings.TrimPrefix(req.URL.Path, "/cancel/")
	if jobID == "" {
		http.Error(rw, "missing job id", http.StatusBadRequest)
		return
	}

	w.mu.Lock()
	handle, ok := w.jobs[jobID]
	w.mu.Unlock()

	if !ok {
		http.Error(rw, "job not found", http.StatusNotFound)
		return
	}

	handle.cancel()
	writeJSON(rw, http.StatusOK, map[string]string{"status": "cancelled", "job_id": jobID})
}

func (w *Worker) handleStatus(rw http.ResponseWriter, req *http.Request) {
	jobID := strings.TrimPrefix(req.URL.Path, "/status/")
	if jobID == "" {
		http.Error(rw, "missing job id", http.StatusBadRequest)
		return
	}

	w.mu.RLock()
	_, active := w.jobs[jobID]
	w.mu.RUnlock()

	status := "not_found"
	if active {
		status = "running"
	} else if w.lastJobID == jobID {
		status = "completed"
	}
	writeJSON(rw, http.StatusOK, map[string]string{"job_id": jobID, "status": status})
}

// Shutdown drains the worker: stops accepting new jobs, waits for active jobs.
func (w *Worker) Shutdown(ctx context.Context) error {
	w.mu.Lock()
	w.capacity = 0 // prevent new jobs
	active := make([]*jobHandle, 0, len(w.jobs))
	for _, h := range w.jobs {
		active = append(active, h)
	}
	w.mu.Unlock()

	// Cancel all active jobs and wait for them to finish (with a timeout from ctx).
	for _, h := range active {
		h.cancel()
	}
	return nil
}

func workerStatus(active, capacity int) string {
	if active >= capacity {
		return "busy"
	}
	if active > 0 {
		return "running"
	}
	return "ready"
}
