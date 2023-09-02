// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bufpools

import "bytes"

const (
	// segmentSize is the size of each segment.
	segmentSize = 64 << 10
	// maxRetainSegmentSlots specifies the maximum number of segment slots
	// to retain after calling Buffer.Reset.
	maxRetainSegmentSlots = 64
)

// Buffer is similar to [bytes.Buffer],
// but uses a series of segmented buffers instead of a single contiguous buffer,
// which are internally retrieved from the buffer pools as needed.
type Buffer struct {
	length   int
	segments [][]byte
}

func (b *Buffer) last() *[]byte {
	return &b.segments[len(b.segments)-1]
}

// Len returns the number of bytes written to the buffer.
func (b *Buffer) Len() (n int) {
	return b.length
}

// Cap returns the capacity of the buffer.
func (b *Buffer) Cap() int {
	// Intermediate segments may technically have unused capacity,
	// but we only ever append to the end, so we only add the available capacity
	// of the last segment to the total length.
	return b.Len() + b.Available()
}

// Available returns how many bytes are unused in the buffer.
func (b *Buffer) Available() int {
	return cap(b.AvailableBuffer())
}

// AvailableBuffer returns an empty buffer with [Buffer.Available] capacity.
// This buffer is intended to be appended to and
// passed to an immediately succeeding [Buffer.Write] call.
// The buffer is only valid until the next mutating operation on b.
// The caller must not retain the returned slice after
// any subsequent call to other Buffer methods.
func (b *Buffer) AvailableBuffer() []byte {
	if len(b.segments) > 0 {
		return (*b.last())[len(*b.last()):]
	}
	return nil
}

// Grow grows the capacity, if necessary, to guarantee space for another n bytes.
func (b *Buffer) Grow(n int) {
	if b.Available() < n {
		if len(b.segments) > 0 && len(*b.last()) == 0 {
			Put(*b.last())
			b.segments = b.segments[:len(b.segments)-1]
		}
		if n <= segmentSize {
			n = segmentSize
		}
		b.segments = append(b.segments, Get(n))
	}
}

// Write appends the contents of p to the buffer, growing the buffer as needed.
func (b *Buffer) Write(p []byte) (int, error) {
	if len(p) > 0 {
		b.Grow(len(p)) // len(b.segments) > 0 after b.Grow
		last := b.last()
		copy((*last)[len(*last):cap(*last)], p)
		*last = (*last)[:len(*last)+len(p)]
		b.length += len(p)
	}
	return len(p), nil
}

// Bytes returns the buffer content as a single contiguous buffer.
// It may need to merge segments to produce a contiguous buffer.
// Unlike [bytes.Buffer.Bytes], this is unsafe for concurrent access.
//
// The buffer is only valid until the next mutating operation on b.
// The caller must not retain the returned slice after
// any subsequent call to other Buffer methods.
//
// If [Buffer.Len] is zero, the returned slice may or may not be nil.
func (b *Buffer) Bytes() []byte {
	if len(b.segments) == 0 {
		return nil
	}
	if len(b.segments) > 1 {
		p := Get(b.Len())
		for i := range b.segments {
			p = append(p, b.segments[i]...)
			Put(b.segments[i])
			b.segments[i] = nil // allow GC to reclaim the buffer
		}
		b.segments = append(b.segments[:0], p)
	}
	return b.segments[0]
}

// BytesClone returns a copy of the buffer content.
//
// If [Buffer.Len] is zero, the returned slice may or may not be nil.
func (b *Buffer) BytesClone() []byte {
	return bytes.Join(b.segments, nil)
}

// Reset resets the buffer to be empty,
// releasing all segments back to the pool,
// except it retains a single empty segment (if available).
// The purpose of retaining a segment is to reduce the cost
// of subsequent usages of Buffer such that it does not need to
// fetch the first segment again.
func (b *Buffer) Reset() {
	var retain []byte // non-nil for segment to retain
	for i := len(b.segments) - 1; i >= 0; i-- {
		p := b.segments[i]
		if retain == nil && cap(p) <= segmentSize {
			p = (p)[:0] // retain locally, but clear the length
			retain = p
		} else {
			Put(p)
		}
		b.segments[i] = nil // allow GC to reclaim the buffer
	}
	b.length = 0
	b.segments = b.segments[:0]
	if cap(b.segments) > maxRetainSegmentSlots {
		b.segments = nil // avoid excessive memory for segment headers
	}
	if retain != nil {
		b.segments = append(b.segments, retain)
	}
}
