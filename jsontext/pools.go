// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsontext

import (
	"bytes"
	"io"
	"sync"

	"github.com/go-json-experiment/json/internal/bufpools"
)

// TODO(https://go.dev/issue/47657): Use sync.PoolOf.

var (
	// This implicitly owns the internal buffer by also owning
	// the bufpools.Buffer used as the underlying io.Writer.
	bufferedEncoderPool = &sync.Pool{New: func() any { return new(Encoder) }}

	// This owns the internal buffer, but it is only used to temporarily store
	// buffered JSON before flushing it to the underlying io.Writer.
	// In a sufficiently efficient streaming mode, we do not expect the buffer
	// to grow arbitrarily large. Thus, we avoid recycling large buffers.
	streamingEncoderPool = &sync.Pool{New: func() any { return new(Encoder) }}

	// This does not own the internal buffer since
	// it is taken directly from the provided bytes.Buffer.
	bytesBufferEncoderPool = &sync.Pool{New: func() any { return new(Encoder) }}
)

func getBufferedEncoder(opts ...Options) *Encoder {
	e := bufferedEncoderPool.Get().(*Encoder)
	if e.s.Wr == nil {
		e.s.Wr = new(bufpools.Buffer)
	}
	e.s.reset(nil, e.s.Wr, opts...)
	return e
}
func putBufferedEncoder(e *Encoder) {
	e.s.Wr.(*bufpools.Buffer).Reset()
	e.s.Buf = nil
	bufferedEncoderPool.Put(e)
}

func getStreamingEncoder(w io.Writer, opts ...Options) *Encoder {
	if _, ok := w.(*bytes.Buffer); ok {
		e := bytesBufferEncoderPool.Get().(*Encoder)
		e.s.reset(nil, w, opts...) // buffer taken from bytes.Buffer
		return e
	} else {
		e := streamingEncoderPool.Get().(*Encoder)
		e.s.reset(e.s.Buf[:0], w, opts...) // preserve existing buffer
		return e
	}
}
func putStreamingEncoder(e *Encoder) {
	if _, ok := e.s.Wr.(*bytes.Buffer); ok {
		bytesBufferEncoderPool.Put(e)
	} else {
		if cap(e.s.Buf) > 64<<10 {
			e.s.Buf = nil // avoid pinning arbitrarily large amounts of memory
		}
		streamingEncoderPool.Put(e)
	}
}

var (
	// This does not own the internal buffer since it is externally provided.
	bufferedDecoderPool = &sync.Pool{New: func() any { return new(Decoder) }}

	// This owns the internal buffer, but it is only used to temporarily store
	// buffered JSON fetched from the underlying io.Reader.
	// In a sufficiently efficient streaming mode, we do not expect the buffer
	// to grow arbitrarily large. Thus, we avoid recycling large buffers.
	streamingDecoderPool = &sync.Pool{New: func() any { return new(Decoder) }}

	// This does not own the internal buffer since
	// it is taken directly from the provided bytes.Buffer.
	bytesBufferDecoderPool = bufferedDecoderPool
)

func getBufferedDecoder(b []byte, opts ...Options) *Decoder {
	d := bufferedDecoderPool.Get().(*Decoder)
	d.s.reset(b, nil, opts...)
	return d
}
func putBufferedDecoder(d *Decoder) {
	bufferedDecoderPool.Put(d)
}

func getStreamingDecoder(r io.Reader, opts ...Options) *Decoder {
	if _, ok := r.(*bytes.Buffer); ok {
		d := bytesBufferDecoderPool.Get().(*Decoder)
		d.s.reset(nil, r, opts...) // buffer taken from bytes.Buffer
		return d
	} else {
		d := streamingDecoderPool.Get().(*Decoder)
		d.s.reset(d.s.buf[:0], r, opts...) // preserve existing buffer
		return d
	}
}
func putStreamingDecoder(d *Decoder) {
	if _, ok := d.s.rd.(*bytes.Buffer); ok {
		bytesBufferDecoderPool.Put(d)
	} else {
		if cap(d.s.buf) > 64<<10 {
			d.s.buf = nil // avoid pinning arbitrarily large amounts of memory
		}
		streamingDecoderPool.Put(d)
	}
}
