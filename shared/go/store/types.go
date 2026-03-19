package store

// JobStatus is the typed representation of the jobs.status column.
// This file is hand-written and is never touched by sqlc generate.
type JobStatus string

const (
	JobStatusQueued     JobStatus = "queued"
	JobStatusRunning    JobStatus = "running"
	JobStatusSucceeded  JobStatus = "succeeded"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCancelled  JobStatus = "cancelled"
	JobStatusSuperseded JobStatus = "superseded"
)

// IsTerminal returns true if the job is in a final state and will never run again.
func (s JobStatus) IsTerminal() bool {
	switch s {
	case JobStatusSucceeded, JobStatusFailed, JobStatusCancelled, JobStatusSuperseded:
		return true
	}
	return false
}
