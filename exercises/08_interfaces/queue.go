package interfaces

// Job is a unit of work with a scheduling priority.
type Job struct {
	Name     string
	Priority int
}

// ByPriority orders jobs for sort.Interface: higher Priority first, ties broken
// by Name ascending (Part 6).
type ByPriority []Job

// Len implements sort.Interface.
func (p ByPriority) Len() int { return len(p) }

// Less implements sort.Interface.
func (p ByPriority) Less(i, j int) bool { return false }

// Swap implements sort.Interface.
func (p ByPriority) Swap(i, j int) {}

// Queue is a max-priority queue backed by container/heap (Part 6). Equal
// priorities pop in FIFO order. Its zero value is ready to use.
type Queue struct{}

// Push adds a job.
func (q *Queue) Push(j Job) {}

// Pop removes and returns the highest-priority job; ok is false when empty.
func (q *Queue) Pop() (Job, bool) { return Job{}, false }

// Len returns the number of queued jobs.
func (q *Queue) Len() int { return 0 }
