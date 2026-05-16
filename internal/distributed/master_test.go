package distributed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JinkaiLiu/vibeready/internal/config"
)

func TestMasterRunMergesWorkerSnapshots(t *testing.T) {
	t.Parallel()

	worker1 := NewWorker("worker-a", ":0", 1, "")
	worker2 := NewWorker("worker-b", ":0", 1, "")
	server1 := httptest.NewServer(worker1.Handler())
	defer server1.Close()
	server2 := httptest.NewServer(worker2.Handler())
	defer server2.Close()
	target := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("x-ai-output-tokens", "10")
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()

	cfg := config.Config{
		URL:             target.URL,
		Method:          "GET",
		Concurrency:     2,
		Duration:        100 * time.Millisecond,
		Timeout:         500 * time.Millisecond,
		ModelPricePer1K: 0.002,
	}

	master := NewMaster(2*time.Second, "", 0)
	state, err := master.Run(context.Background(), cfg, []string{server1.URL, server2.URL})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if state.WorkerCount != 2 || len(state.Workers) != 2 {
		t.Fatalf("unexpected worker state count: %#v", state)
	}
	if state.Summary.TotalRequests == 0 {
		t.Fatal("expected distributed run to execute requests")
	}
	if state.Summary.TotalRequests != state.Summary.SuccessfulRequests+state.Summary.FailedRequests {
		t.Fatalf("summary counts do not add up: %#v", state.Summary)
	}
	if state.Workers[0].Health.WorkerID == "" || state.Workers[1].Health.WorkerID == "" {
		t.Fatalf("expected health check results to be captured: %#v", state.Workers)
	}
	if state.Summary.TotalOutputTokens == 0 || state.Summary.EstimatedCost == 0 {
		t.Fatalf("expected distributed summary to preserve token cost metrics: %#v", state.Summary)
	}
}

func TestMasterCheckWorkersFailsFast(t *testing.T) {
	t.Parallel()

	worker := NewWorker("worker-a", ":0", 1, "")
	server := httptest.NewServer(worker.Handler())
	defer server.Close()

	master := NewMaster(time.Second, "", 0)
	_, err := master.CheckWorkers(context.Background(), []string{server.URL, "http://127.0.0.1:1"})
	if err == nil {
		t.Fatal("expected health check failure")
	}
}

func TestMasterDashboardLatestEndpoint(t *testing.T) {
	t.Parallel()

	master := NewMaster(time.Second, "", 0)
	master.setState(DashboardState{JobID: "job-1", Status: "running"})

	req := httptest.NewRequest(http.MethodGet, "/api/latest", nil)
	rec := httptest.NewRecorder()
	master.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code %d", rec.Code)
	}
	var state DashboardState
	if err := json.NewDecoder(rec.Body).Decode(&state); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if state.JobID != "job-1" {
		t.Fatalf("unexpected dashboard state: %#v", state)
	}
}

func TestMasterRunShardsRequestsAcrossWorkers(t *testing.T) {
	t.Parallel()

	var worker1Requests atomic.Int64
	var worker2Requests atomic.Int64

	worker1 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/health":
			_ = json.NewEncoder(rw).Encode(WorkerHealth{WorkerID: "w1", Status: "ready"})
		case "/run":
			var job JobRequest
			_ = json.NewDecoder(req.Body).Decode(&job)
			worker1Requests.Store(job.Config.Requests)
			_ = json.NewEncoder(rw).Encode(WorkerResult{WorkerID: "w1", JobID: job.JobID})
		default:
			http.NotFound(rw, req)
		}
	}))
	defer worker1.Close()
	worker2 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/health":
			_ = json.NewEncoder(rw).Encode(WorkerHealth{WorkerID: "w2", Status: "ready"})
		case "/run":
			var job JobRequest
			_ = json.NewDecoder(req.Body).Decode(&job)
			worker2Requests.Store(job.Config.Requests)
			_ = json.NewEncoder(rw).Encode(WorkerResult{WorkerID: "w2", JobID: job.JobID})
		default:
			http.NotFound(rw, req)
		}
	}))
	defer worker2.Close()

	cfg := config.Config{
		URL:         "http://localhost",
		Method:      "GET",
		Concurrency: 1,
		Requests:    5,
		Timeout:     time.Second,
	}
	master := NewMaster(time.Second, "", 0)
	_, err := master.Run(context.Background(), cfg, []string{worker1.URL, worker2.URL})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	total := worker1Requests.Load() + worker2Requests.Load()
	if total != 5 {
		t.Fatalf("expected sharded requests to sum to 5, got %d", total)
	}
}

func TestMasterCheckWorkersRejectsBusyWorker(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_ = json.NewEncoder(rw).Encode(WorkerHealth{WorkerID: "busy-1", Status: "busy", Capacity: 1, ActiveJobs: 1})
	}))
	defer server.Close()

	master := NewMaster(time.Second, "", 0)
	_, err := master.CheckWorkers(context.Background(), []string{server.URL})
	if err == nil {
		t.Fatal("expected busy worker to be rejected")
	}
}

func TestJobQueueEnqueueDequeue(t *testing.T) {
	t.Parallel()

	q := NewJobQueue(10)
	id, err := q.Enqueue(JobSpec{Workers: []string{"http://w1:8081"}})
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty job id")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	qj, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue returned error: %v", err)
	}
	if qj.ID != id {
		t.Fatalf("dequeued job id mismatch: %s != %s", qj.ID, id)
	}
}

func TestJobQueueComplete(t *testing.T) {
	t.Parallel()

	q := NewJobQueue(10)
	id, _ := q.Enqueue(JobSpec{Workers: []string{"http://w1:8081"}})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, _ = q.Dequeue(ctx)
	q.Complete(id, DashboardState{JobID: id, Status: "completed"})

	info, ok := q.Get(id)
	if !ok {
		t.Fatal("job not found after completion")
	}
	if info.Status != JobCompleted {
		t.Fatalf("expected completed status, got %s", info.Status)
	}
}

func TestJobQueueCancel(t *testing.T) {
	t.Parallel()

	q := NewJobQueue(10)
	id, _ := q.Enqueue(JobSpec{Workers: []string{"http://w1:8081"}})

	if err := q.Cancel(id); err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}

	info, ok := q.Get(id)
	if !ok {
		t.Fatal("job not found after cancel")
	}
	if info.Status != JobCancelled {
		t.Fatalf("expected cancelled status, got %s", info.Status)
	}
}

func TestJobQueueCancelIsTerminal(t *testing.T) {
	t.Parallel()

	q := NewJobQueue(10)
	id, _ := q.Enqueue(JobSpec{Workers: []string{"http://w1:8081"}})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := q.Dequeue(ctx); err != nil {
		t.Fatalf("Dequeue returned error: %v", err)
	}
	if err := q.Cancel(id); err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}

	q.Complete(id, DashboardState{JobID: id, Status: "completed"})
	info, ok := q.Get(id)
	if !ok {
		t.Fatal("job not found")
	}
	if info.Status != JobCancelled {
		t.Fatalf("cancelled job should not be completed, got %s", info.Status)
	}

	q.Fail(id, context.Canceled)
	info, ok = q.Get(id)
	if !ok {
		t.Fatal("job not found")
	}
	if info.Status != JobCancelled {
		t.Fatalf("cancelled job should not be failed, got %s", info.Status)
	}
}

func TestJobQueueList(t *testing.T) {
	t.Parallel()

	q := NewJobQueue(10)
	q.Enqueue(JobSpec{Workers: []string{"http://w1:8081"}})
	q.Enqueue(JobSpec{Workers: []string{"http://w2:8081"}})

	jobs := q.List()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs in list, got %d", len(jobs))
	}
}

func TestWorkerCapacitySlots(t *testing.T) {
	t.Parallel()

	worker := NewWorker("multi", ":0", 2, "")
	server := httptest.NewServer(worker.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()
	var health WorkerHealth
	json.NewDecoder(resp.Body).Decode(&health)
	if health.Capacity != 2 {
		t.Fatalf("expected capacity 2, got %d", health.Capacity)
	}
	if health.ActiveJobs != 0 {
		t.Fatalf("expected 0 active jobs, got %d", health.ActiveJobs)
	}
}

func TestWorkerCancelEndpoint(t *testing.T) {
	t.Parallel()

	worker := NewWorker("cancel-test", ":0", 1, "")
	server := httptest.NewServer(worker.Handler())
	defer server.Close()

	resp, err := http.Post(server.URL+"/cancel/nonexistent", "application/json", nil)
	if err != nil {
		t.Fatalf("cancel request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing job, got %d", resp.StatusCode)
	}
}

func TestWorkerStatusEndpoint(t *testing.T) {
	t.Parallel()

	worker := NewWorker("status-test", ":0", 1, "")
	server := httptest.NewServer(worker.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/status/nonexistent")
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "not_found" {
		t.Fatalf("expected not_found, got %s", result["status"])
	}
}

func TestAuthSignAndVerify(t *testing.T) {
	secret := "test-secret"
	jobID := "job-123"

	sig := SignJob(jobID, secret)
	if sig == "" {
		t.Fatal("expected non-empty signature")
	}

	req, _ := http.NewRequest(http.MethodPost, "/run", nil)
	req.Header.Set("Authorization", sig)
	if !VerifyJob(req, jobID, secret) {
		t.Fatal("expected valid signature")
	}

	if VerifyJob(req, jobID, "wrong-secret") {
		t.Fatal("expected wrong secret to fail verification")
	}

	if VerifyJob(req, "job-456", secret) {
		t.Fatal("expected wrong job ID to fail verification")
	}
}

func TestAuthEmptySecret(t *testing.T) {
	if sig := SignJob("job-123", ""); sig != "" {
		t.Fatal("expected empty signature for empty secret")
	}
	req, _ := http.NewRequest(http.MethodPost, "/run", nil)
	if !VerifyJob(req, "job-123", "") {
		t.Fatal("expected empty secret to always verify")
	}
}

func TestMasterPersistentAPI(t *testing.T) {
	t.Parallel()

	w1 := NewWorker("w1", ":0", 2, "")
	s1 := httptest.NewServer(w1.Handler())
	defer s1.Close()
	w2 := NewWorker("w2", ":0", 2, "")
	s2 := httptest.NewServer(w2.Handler())
	defer s2.Close()

	target := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()

	master := NewMaster(5*time.Second, "", 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	master.StartBackground(ctx)

	spec := JobSpec{
		Config: config.Config{
			URL:         target.URL,
			Method:      "GET",
			Concurrency: 1,
			Duration:    50 * time.Millisecond,
			Timeout:     500 * time.Millisecond,
		},
		Workers: []string{s1.URL, s2.URL},
	}

	id, err := master.SubmitJob(spec)
	if err != nil {
		t.Fatalf("SubmitJob returned error: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty job id")
	}

	time.Sleep(500 * time.Millisecond)

	info, ok := master.GetJob(id)
	if !ok {
		t.Fatal("job not found")
	}
	if info.Status != JobCompleted {
		t.Fatalf("expected job to complete, got status %s", info.Status)
	}
	if info.Summary == nil {
		t.Fatal("expected summary in completed job")
	}
}

func TestMasterHandleJobsAPI(t *testing.T) {
	t.Parallel()

	master := NewMaster(time.Second, "", 2)
	handler := master.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(`invalid`))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
