// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"io"
	"math/bits"
	"sync"
)

// TODO(https://golang.org/issue/47657): Use sync.PoolOf.

var encoderPool = sync.Pool{New: func() any { return new(Encoder) }}

func getEncoder(b []byte, w io.Writer, o EncodeOptions) *Encoder {
	e := encoderPool.Get().(*Encoder)
	e.reset(b, w, o)
	return e
}
func putEncoder(e *Encoder) {
	*e = Encoder{state: e.state}
	encoderPool.Put(e)
}

var decoderPool = sync.Pool{New: func() any { return new(Decoder) }}

func getDecoder(b []byte, r io.Reader, o DecodeOptions) *Decoder {
	d := decoderPool.Get().(*Decoder)
	d.reset(b, r, o)
	return d
}
func putDecoder(d *Decoder) {
	*d = Decoder{state: d.state, stringCache: d.stringCache}
	decoderPool.Put(d)
}

// bufferPool is a pool of variable-length buffers.
//
// Example usage:
//
//	b := getBuffer()
//	defer putBuffer(b)
//	b.buf = appendFoo(b.buf, foo)        // may resize b.buf to arbitrarily large sizes
//	return append([]byte(nil), b.buf...) // single copy of the b.buf contents
//
// It avoids https://golang.org/issue/23199 by locally tracking
// statistics on the utilization of the buffer to avoid
// pinning arbitrarily large buffers on the heap forever.
var bufferPool = sync.Pool{
	New: func() any { return new(pooledBuffer) },
}

type pooledBuffer struct {
	buf     []byte
	strikes int // number of times the buffer was under-utilized
	prevLen int // length of previous buffer
}

// getBuffer retrieves a buffer from the pool,
// where len(b.buf) is guaranteed to be zero and cap(b.buf) > 0.
func getBuffer() (b *pooledBuffer) {
	b = bufferPool.Get().(*pooledBuffer)
	if b.buf == nil {
		// Round up to nearest 2ⁿ to make best use of malloc size classes.
		// See runtime/sizeclasses.go on Go1.15.
		// Logical OR with 63 to ensure 64 as the minimum buffer size.
		n := 1 << bits.Len(uint(b.prevLen|63))
		b.buf = make([]byte, 0, n)
	}
	return b
}

// putBuffer places the buffer back into the pool,
// where len(b.buf) is the actual amount of the buffer that was used.
func putBuffer(b *pooledBuffer) {
	// Recycle large buffers only if sufficiently utilized.
	// If a buffer is under-utilized enough times sequentially,
	// then it is discarded, ensuring that a single large buffer
	// won't be kept alive by a continuous stream of small usages.
	//
	// The worst case utilization is computed as:
	//	MIN_UTILIZATION_THRESHOLD / (1 + MAX_NUM_STRIKES)
	//
	// For the constants chosen below, this is (25%)/(1+4) ⇒ 5%.
	// This may seem low, but it ensures a lower bound on
	// the absolute worst-case utilization. Without this check,
	// this would be theoretically 0%, which is infinitely worse.
	//
	// See https://golang.org/issue/27735.
	switch {
	case cap(b.buf) <= 4<<10: // always recycle buffers smaller than 4KiB
		b.strikes = 0
	case cap(b.buf)/4 <= len(b.buf): // at least 25% utilization
		b.strikes = 0
	case b.strikes < 4: // at most 4 strikes
		b.strikes++
	default: // discard the buffer; too large and too often under-utilized
		b.strikes = 0
		b.prevLen = len(b.buf) // heuristic for size to allocate next time
		b.buf = nil
	}
	b.buf = b.buf[:0]
	bufferPool.Put(b)
}
