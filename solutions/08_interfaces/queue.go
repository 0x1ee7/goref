package interfaces

import "container/heap"

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
func (p ByPriority) Less(i, j int) bool {
	if p[i].Priority != p[j].Priority {
		return p[i].Priority > p[j].Priority
	}
	return p[i].Name < p[j].Name
}

// Swap implements sort.Interface.
func (p ByPriority) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// queueItem pairs a job with its arrival sequence so ties pop in FIFO order.
type queueItem struct {
	job Job
	seq uint64
}

// jobHeap implements heap.Interface as a max-heap on priority, FIFO on ties.
type jobHeap []queueItem

func (h jobHeap) Len() int { return len(h) }

func (h jobHeap) Less(i, j int) bool {
	if h[i].job.Priority != h[j].job.Priority {
		return h[i].job.Priority > h[j].job.Priority
	}
	return h[i].seq < h[j].seq
}

func (h jobHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *jobHeap) Push(x any) { *h = append(*h, x.(queueItem)) }

func (h *jobHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// Queue is a max-priority queue backed by container/heap (Part 6). Equal
// priorities pop in FIFO order. Its zero value is ready to use.
type Queue struct {
	h   jobHeap
	seq uint64
}

// Push adds a job.
func (q *Queue) Push(j Job) {
	heap.Push(&q.h, queueItem{job: j, seq: q.seq})
	q.seq++
}

// Pop removes and returns the highest-priority job; ok is false when empty.
func (q *Queue) Pop() (Job, bool) {
	if len(q.h) == 0 {
		return Job{}, false
	}
	return heap.Pop(&q.h).(queueItem).job, true
}

// Len returns the number of queued jobs.
func (q *Queue) Len() int { return len(q.h) }
