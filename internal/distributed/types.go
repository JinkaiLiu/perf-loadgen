package distributed

import (
	"time"

	"github.com/JinkaiLiu/perf-loadgen/internal/config"
	"github.com/JinkaiLiu/perf-loadgen/pkg/types"
)

// JobRequest is sent by master to each worker.
type JobRequest struct {
	JobID  string        `json:"job_id"`
	Config config.Config `json:"config"`
}

// WorkerResult is the response returned by a worker after completing a job.
type WorkerResult struct {
	WorkerID string                  `json:"worker_id"`
	JobID    string                  `json:"job_id"`
	Started  time.Time               `json:"started"`
	Ended    time.Time               `json:"ended"`
	Snapshot types.AggregateSnapshot `json:"snapshot"`
	Summary  types.Summary           `json:"summary"`
	Error    string                  `json:"error,omitempty"`
}

// WorkerHealth summarizes the worker process state.
type WorkerHealth struct {
	WorkerID     string    `json:"worker_id"`
	Status       string    `json:"status"`
	Busy         bool      `json:"busy"`
	CurrentJobID string    `json:"current_job_id,omitempty"`
	LastJobID    string    `json:"last_job_id,omitempty"`
	LastSeen     time.Time `json:"last_seen"`
	ListenAddr   string    `json:"listen_addr"`
	Capacity     int       `json:"capacity"`
	ActiveJobs   int       `json:"active_jobs"`
	JobIDs       []string  `json:"job_ids,omitempty"`
}

// WorkerState captures the master's view of a single worker for the dashboard.
type WorkerState struct {
	WorkerID string       `json:"worker_id"`
	Address  string       `json:"address"`
	Status   string       `json:"status"`
	Health   WorkerHealth `json:"health,omitempty"`
	Result   WorkerResult `json:"result,omitempty"`
	Error    string       `json:"error,omitempty"`
}

// DashboardState is the latest distributed run shown by the master dashboard.
type DashboardState struct {
	JobID       string        `json:"job_id"`
	Status      string        `json:"status"`
	Started     time.Time     `json:"started"`
	Completed   time.Time     `json:"completed,omitempty"`
	Summary     types.Summary `json:"summary"`
	Workers     []WorkerState `json:"workers"`
	WorkerCount int           `json:"worker_count"`
}

// JobStatus represents the lifecycle state of a distributed job.
type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

// JobSpec is the payload submitted by clients to create a job.
type JobSpec struct {
	Config  config.Config `json:"config"`
	Workers []string      `json:"workers"`
}

// JobInfo describes a job in the master's API.
type JobInfo struct {
	ID        string          `json:"id"`
	Status    JobStatus       `json:"status"`
	Workers   []string        `json:"workers"`
	CreatedAt time.Time       `json:"created_at"`
	StartedAt time.Time       `json:"started_at,omitempty"`
	EndedAt   time.Time       `json:"ended_at,omitempty"`
	Result    DashboardState  `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	Summary   *types.Summary  `json:"summary,omitempty"`
}

// WorkerCapacityInfo is the per-worker capacity view used by the balancer.
type WorkerCapacityInfo struct {
	Address        string
	Capacity       int
	ActiveJobs     int
	AvailableSlots int
}
