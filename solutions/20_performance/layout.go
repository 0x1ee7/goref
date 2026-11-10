package perf

import "strconv"

// Sample is one measurement. Fields are ordered largest first so that no
// padding is needed: 8+8+2+1+1 = 20 bytes of data, padded to 24 by the 8-byte
// alignment of the struct. The naive order (bool, uint64, uint16, int64, uint8)
// would pad to 40 bytes, because every small field sits between 8-byte fields.
type Sample struct {
	ID      uint64
	Latency int64 // nanoseconds
	Status  uint16
	Retries uint8
	OK      bool
}

// AppendSample appends a fixed-format, newline-terminated rendering of s to dst:
//
//	id=<ID> latency=<Latency> status=<Status> retries=<Retries> ok=<true|false>\n
//
// It must not allocate when dst has enough spare capacity, and it must not use
// fmt, whose interface arguments box every value.
func AppendSample(dst []byte, s Sample) []byte {
	dst = append(dst, "id="...)
	dst = strconv.AppendUint(dst, s.ID, 10)
	dst = append(dst, " latency="...)
	dst = strconv.AppendInt(dst, s.Latency, 10)
	dst = append(dst, " status="...)
	dst = strconv.AppendUint(dst, uint64(s.Status), 10)
	dst = append(dst, " retries="...)
	dst = strconv.AppendUint(dst, uint64(s.Retries), 10)
	dst = append(dst, " ok="...)
	dst = strconv.AppendBool(dst, s.OK)
	return append(dst, '\n')
}
