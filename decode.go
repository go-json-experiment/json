// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import "io"

// Decoder is a streaming decoder for raw JSON values and tokens.
//
// ReadToken and ReadValue calls may be interleaved.
// For example, the following JSON value:
//
//	{"key":"value","array":[null,false,true,3.14159],"object":{"k":"v"}}
//
// can be parsed with the following calls (ignoring errors for brevity):
//
//	d.ReadToken() // {
//	d.ReadToken() // "key"
//	d.ReadToken() // "value"
//	d.ReadValue() // "array"
//	d.ReadToken() // [
//	d.ReadToken() // null
//	d.ReadToken() // false
//	d.ReadValue() // true
//	d.ReadToken() // 3.14159
//	d.ReadToken() // ]
//	d.ReadValue() // "object"
//	d.ReadValue() // {"k":"v"}
//	d.ReadToken() // }
//
// The above is one of many possible sequence of calls and
// may not represent the most sensible method to call for any given token/value.
// For example, it is probably more common to call ReadToken to obtain a
// string token for object names.
type Decoder struct{}

// NewDecoder constructs a new streaming decoder reading from r.
func NewDecoder(r io.Reader) *Decoder {
	panic("not implemented")
}

// PeekKind retrieves the next token kind, but does not advance the read offset.
// It returns 0 if there are no more tokens.
// It returns an invalid non-zero kind if an error occurs.
func (d *Decoder) PeekKind() Kind {
	panic("not implemented")
}

// ReadToken reads the next Token, advancing the read offset.
// The returned token is only valid until the next Peek or Read call.
// It returns io.EOF if there are no more tokens.
func (d *Decoder) ReadToken() (Token, error) {
	// TODO: May the user allow Token to escape?
	panic("not implemented")
}

// ReadValue returns the next raw JSON value, advancing the read offset.
// The value is stripped of any leading or trailing whitespace.
// The returned value is only valid until the next Peek or Read call and
// may not be mutated while the Decoder remains in use.
// It returns io.EOF if there are no more values.
func (d *Decoder) ReadValue() (Value, error) {
	panic("not implemented")
}

// TODO: How should internal buffering work? For performance, Decoder will read
//	decently sized chunks, which may cause it read past the next JSON value.
//	In v1, json.Decoder.Buffered provides access to the excess data.
//	Alternatively, we could take the "compress/flate" approach and guarantee
//	that we never read past the last delimiter if the provided io.Reader
//	also implements io.ByteReader.
