// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import "reflect"

var (
	encoderPointerType   = reflect.TypeOf((*Encoder)(nil))
	decoderPointerType   = reflect.TypeOf((*Decoder)(nil))
	marshalOptionsType   = reflect.TypeOf((*MarshalOptions)(nil)).Elem()
	unmarshalOptionsType = reflect.TypeOf((*UnmarshalOptions)(nil)).Elem()
	marshalersType       = reflect.TypeOf((*Marshalers)(nil)).Elem()
	unmarshalersType     = reflect.TypeOf((*Unmarshalers)(nil)).Elem()
	bytesType            = reflect.TypeOf((*[]byte)(nil)).Elem()
	errorType            = reflect.TypeOf((*error)(nil)).Elem()
)

// SkipFunc may be returned by custom marshal and unmarshal functions
// that operate on an Encoder or Decoder.
//
// Any function that returns SkipFunc must not cause observable side effects
// on the provided Encoder or Decoder. For example, it is permissible to call
// Decoder.PeekKind, but not permissible to call Decoder.ReadToken or
// Encoder.WriteToken since such methods mutate the state.
const SkipFunc = jsonError("skip function")

// Marshalers is a list of functions that may override the marshal behavior
// of specific types. Populate MarshalOptions.Marshalers to use it.
// A nil *Marshalers is equivalent to an empty list.
type Marshalers struct{}

// NewMarshalers constructs a list of marshal functions to override
// the marshal behavior for specific types.
//
// Each input must be a function with one the following signatures:
//
//	func(T) ([]byte, error)
//	func(*Encoder, MarshalOptions, T) error
//
// A marshal function operating on an Encoder may return SkipFunc to signal
// that the function is to be skipped and that the next function be used.
//
// The input may also include *Marshalers values, which is equivalent to
// inlining the list of marshal functions used to construct it.
func NewMarshalers(fns ...interface{}) *Marshalers {
	// TODO: Document what T may be and the guarantees
	// for the values passed to custom marshalers.
	panic("not implemented")
}

// Unmarshalers is a list of functions that may override the unmarshal behavior
// of specific types. Populate UnmarshalOptions.Unmarshalers to use it.
// A nil *Unmarshalers is equivalent to an empty list.
type Unmarshalers struct{}

// NewUnmarshalers constructs a list of unmarshal functions to override
// the unmarshal behavior for specific types.
//
// Each input must be a function with one the following signatures:
//
//	func([]byte, T) error
//	func(*Decoder, UnmarshalOptions, T) error
//
// An unmarshal function operating on a Decoder may return SkipFunc to signal
// that the function is to be skipped and that the next function be used.
//
// The input may also include *Unmarshalers values, which is equivalent to
// inlining the list of unmarshal functions used to construct it.
func NewUnmarshalers(fns ...interface{}) *Unmarshalers {
	// TODO: Document what T may be and the guarantees
	// for the values passed to custom unmarshalers.
	panic("not implemented")
}
