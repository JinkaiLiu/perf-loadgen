package distributed

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// queuedJob is a job waiting in or being processed by the master.
type queuedJob struct {
	ID        string
	Spec      JobSpec
	Status    JobStatus
	CreatedAt time.Time
	StartedAt time.Time
	EndedAt   time.Time
	Result    DashboardState
	Error     string
	Cancel    context.CancelFunc
}

// JobQueue is a bounded FIFO job queue with cancellation support.
type JobQueue struct {
	mu       sync.RWMutex
	jobs     map[string]*queuedJob
	order    []string
	maxJobs  int
	notEmpty chan struct{}
}

// NewJobQueue creates a bounded job queue.
func NewJobQueue(maxJobs int) *JobQueue {
	if maxJobs <= 0 {
		maxJobs = 100
	}
	return &JobQueue{
		jobs:     make(map[string]*queuedJob),
		maxJobs:  maxJobs,
		notEmpty: make(chan struct{}, 1),
	}
}

// Enqueue adds a job to the queue. Returns error if the queue is full.
func (q *JobQueue) Enqueue(spec JobSpec) (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.jobs) >= q.maxJobs {
		return "", fmt.Errorf("job queue full (%d/%d)", len(q.jobs), q.maxJobs)
	}

	id := fmt.Sprintf("job-%d", time.Now().UnixNano())
	qj := &queuedJob{
		ID:        id,
		Spec:      spec,
		Status:    JobPending,
		CreatedAt: time.Now(),
	}
	q.jobs[id] = qj
	q.order = append(q.order, id)

	select {
	case q.notEmpty <- struct{}{}:
	default:
	}
	return id, nil
}

// Dequeue blocks until a pending job is available, returns it, and
// atomically attaches cancel so Cancel() sees a non-nil func immediately.
func (q *JobQueue) Dequeue(ctx context.Context, cancel context.CancelFunc) (*queuedJob, error) {
	for {
		q.mu.Lock()
		for _, id := range q.order {
			qj := q.jobs[id]
			if qj.Status == JobPending {
				qj.Status = JobRunning
				qj.StartedAt = time.Now()
				qj.Cancel = cancel
				q.mu.Unlock()
				return qj, nil
			}
		}
		q.mu.Unlock()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-q.notEmpty:
		}
	}
}

// Complete marks a job as completed.
func (q *JobQueue) Complete(id string, result DashboardState) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if qj, ok := q.jobs[id]; ok {
		if qj.Status == JobCancelled {
			return
		}
		qj.Status = JobCompleted
		qj.EndedAt = time.Now()
		qj.Result = result
	}
}

// Fail marks a job as failed.
func (q *JobQueue) Fail(id string, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if qj, ok := q.jobs[id]; ok {
		if qj.Status == JobCancelled {
			return
		}
		qj.Status = JobFailed
		qj.EndedAt = time.Now()
		qj.Error = err.Error()
	}
}

// Cancel cancels a running or pending job.
func (q *JobQueue) Cancel(id string) error {
	q.mu.Lock()
	qj, ok := q.jobs[id]
	if !ok {
		q.mu.Unlock()
		return fmt.Errorf("job %s not found", id)
	}
	if qj.Status == JobCompleted || qj.Status == JobFailed || qj.Status == JobCancelled {
		q.mu.Unlock()
		return fmt.Errorf("job %s already in terminal state %s", id, qj.Status)
	}
	qj.Status = JobCancelled
	qj.EndedAt = time.Now()
	if qj.Cancel != nil {
		qj.Cancel()
	}
	q.mu.Unlock()
	return nil
}

// Get returns job info by ID.
func (q *JobQueue) Get(id string) (*JobInfo, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	qj, ok := q.jobs[id]
	if !ok {
		return nil, false
	}
	return q.toJobInfo(qj), true
}

// List returns all jobs ordered by creation time.
func (q *JobQueue) List() []*JobInfo {
	q.mu.RLock()
	defer q.mu.RUnlock()
	result := make([]*JobInfo, 0, len(q.order))
	for _, id := range q.order {
		qj := q.jobs[id]
		result = append(result, q.toJobInfo(qj))
	}
	return result
}

func (q *JobQueue) toJobInfo(qj *queuedJob) *JobInfo {
	info := &JobInfo{
		ID:        qj.ID,
		Status:    qj.Status,
		Workers:   qj.Spec.Workers,
		CreatedAt: qj.CreatedAt,
		StartedAt: qj.StartedAt,
		EndedAt:   qj.EndedAt,
		Result:    qj.Result,
		Error:     qj.Error,
	}
	if qj.Status == JobCompleted || qj.Status == JobFailed {
		info.Summary = &qj.Result.Summary
	}
	return info
}
