// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bufpools implements a pool of buffers and provides a Buffer type
// to manage getting individual buffers from the pool and putting them back.
package bufpools

import (
	"math/bits"
	"sync"
)

const (
	minPooledSegmentShift = 12 // minimum shift size of buffer to pool
	numPools              = bits.UintSize - minPooledSegmentShift
)

// TODO(https://go.dev/issue/47657): Use sync.PoolOf.
// You cannot put a []byte into a pool without it allocating every time
// just to store the slice header. Thus, we have a second pool
// just to cache the use of slice headers. This is silly and
// demonstrates the flaws of the non-generic sync.Pool API.
var sliceHeaderPool = sync.Pool{New: func() any { return new([]byte) }}

// bufferPools is a list of buffer pools.
// Each pool manages buffers of capacity within [1<<shift : 2<<shift),
// where shift is (minPooledSegmentShift+index).
var bufferPools [numPools]sync.Pool

// Get acquires an empty buffer with enough capacity to hold n bytes.
// The unused buffer content is not guaranteed to be zeroed.
func Get(n int) []byte {
	if n < 1<<minPooledSegmentShift {
		n = 1 << minPooledSegmentShift
	}
	shift := bits.Len(uint(n - 1))
	if p, _ := bufferPools[shift-minPooledSegmentShift].Get().(*[]byte); p != nil {
		b := (*p)[:0]
		*p = nil
		sliceHeaderPool.Put(p)
		return b
	}
	return make([]byte, 0, 1<<shift)
}

// Put releases a buffer back to the pools.
// The slice need not be originally retrieved by [Get],
// but the caller must relinquish ownership of the slice.
func Put(b []byte) {
	if cap(b) < 1<<minPooledSegmentShift {
		return
	}
	// TODO: In race detector mode, asynchronously write to the buffer
	// to detect buffers that may have accidentally leaked to users.
	// See https://go.dev/issue/58452 for inspiration.
	p := sliceHeaderPool.Get().(*[]byte)
	*p = b
	shift := bits.Len(uint(cap(b)) - 1)
	bufferPools[shift-minPooledSegmentShift].Put(p)
}
