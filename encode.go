// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import "io"

// Encoder is a streaming encoder for raw JSON values and tokens.
//
// WriteToken and WriteValue calls may be interleaved.
// For example, the following JSON value:
//
//	{"key":"value","array":[null,false,true,3.14159],"object":{"k":"v"}}
//
// can be composed with the following calls (ignoring errors for brevity):
//
//	e.WriteToken(StartObject)        // {
//	e.WriteToken(String("key"))      // "key"
//	e.WriteToken(String("value"))    // "value"
//	e.WriteValue(Value(`"array"`))   // "array"
//	e.WriteToken(StartArray)         // [
//	e.WriteToken(Null)               // null
//	e.WriteToken(False)              // false
//	e.WriteValue(Value("true"))      // true
//	e.WriteToken(Number(3.14159))    // 3.14159
//	e.WriteToken(EndArray)           // ]
//	e.WriteValue(Value(`"object"`))  // "object"
//	e.WriteValue(Value(`{"k":"v"}`)) // {"k":"v"}
//	e.WriteToken(EndObject)          // }
//
// The above is one of many possible sequence of calls and
// may not represent the most sensible method to call for any given token/value.
// For example, it is probably more common to call WriteToken with a string
// for object names.
type Encoder struct{}

// NewEncoder constructs a new streaming encoder writing to w.
// It flushes the internal buffer when the buffer is sufficiently full or
// when a top-level value has been written.
func NewEncoder(w io.Writer) *Encoder {
	panic("not implemented")
}

// WriteToken writes the next Token and advances the internal write offset.
func (e *Encoder) WriteToken(t Token) error {
	// TODO: May the Encoder alias the provided Token?
	panic("not implemented")
}

// WriteValue writes the next Value and advances the internal write offset.
func (e *Encoder) WriteValue(v Value) error {
	// TODO: May the Encoder alias the provided Value?
	panic("not implemented")
}
