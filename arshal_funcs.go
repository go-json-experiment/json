// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

// SkipFunc may be returned by custom marshal and unmarshal functions
// that operate on an Encoder or Decoder.
//
// Any function that returns SkipFunc must not cause observable side effects
// on the provided Encoder or Decoder. For example, it is permissible to call
// Decoder.PeekKind, but not permissible to call Decoder.ReadToken or
// Encoder.WriteToken since such methods mutate the state.
const SkipFunc = jsonError("skip function")

// MarshalerV1 is implemented by types that can marshal themselves.
// It is recommended that types implement MarshalerV2 unless
// the implementation is trying to avoid a hard dependency on this package.
type MarshalerV1 interface {
	MarshalJSON() ([]byte, error)
}

// MarshalerV2 is implemented by types that can marshal themselves.
// It is recommended that types implement MarshalerV2 instead of MarshalerV1
// since this is both more performant and flexible.
// If a type implements both MarshalerV1 and MarshalerV2,
// then MarshalerV2 takes precedence. In such a case, both implementations
// should aim to have equivalent behavior for the default marshal options.
//
// The implementation must write only one JSON value to the Encoder.
type MarshalerV2 interface {
	MarshalNextJSON(*Encoder, MarshalOptions) error

	// TODO: Should users call the MarshalOptions.MarshalNext method or
	// should/can they call this method directly? Does it matter?
}

// Marshalers is a list of functions that may override the marshal behavior
// of specific types. Populate MarshalOptions.Marshalers to use it.
// A nil *Marshalers is equivalent to an empty list.
type Marshalers struct{}

// NewMarshalers constructs a list of marshal functons to override
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

// UnmarshalerV1 is implemented by types that can unmarshal themselves.
// It is recommended that types implement UnmarshalerV2 unless
// the implementation is trying to avoid a hard dependency on this package.
//
// The input can be assumed to be a valid encoding of a JSON value.
// UnmarshalJSON must copy the JSON data if it is retained after returning.
// It is recommended that UnmarshalJSON implement merge semantics when
// unmarshaling into a pre-populated value.
type UnmarshalerV1 interface {
	UnmarshalJSON([]byte) error
}

// UnmarshalerV2 is implemented by types that can marshal themselves.
// It is recommended that types implement UnmarshalerV2 instead of UnmarshalerV1
// since this is both more performant and flexible.
// If a type implements both UnmarshalerV1 and UnmarshalerV2,
// then UnmarshalerV2 takes precedence. In such a case, both implementations
// should aim to have equivalent behavior for the default unmarshal options.
//
// The implementation must read only one JSON value from the Decoder.
// It is recommended that UnmarshalNextJSON implement merge semantics when
// unmarshaling into a pre-populated value.
type UnmarshalerV2 interface {
	UnmarshalNextJSON(*Decoder, UnmarshalOptions) error

	// TODO: Should users call the UnmarshalOptions.UnmarshalNext method or
	// should/can they call this method directly? Does it matter?
}

// Unmarshalers is a list of functions that may override the unmarshal behavior
// of specific types. Populate UnmarshalOptions.Unmarshalers to use it.
// A nil *Unmarshalers is equivalent to an empty list.
type Unmarshalers struct{}

// NewUnmarshalers constructs a list of unmarshal functons to override
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
