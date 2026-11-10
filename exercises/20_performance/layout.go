package perf

// Sample is one measurement. Its fields are declared in a naive order that
// leaves padding; the layout test expects a smaller size.
type Sample struct {
	OK      bool
	ID      uint64
	Status  uint16
	Latency int64 // nanoseconds
	Retries uint8
}

// AppendSample appends a fixed-format, newline-terminated rendering of s to dst:
//
//	id=<ID> latency=<Latency> status=<Status> retries=<Retries> ok=<true|false>\n
//
// It must not allocate when dst has enough spare capacity, and it must not use
// fmt, whose interface arguments box every value.
func AppendSample(dst []byte, s Sample) []byte {
	return dst
}
