// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"io"
)

// MarshalOptions configures how Go data is serialized as JSON data.
// The zero value is equivalent to the default marshal settings.
type MarshalOptions struct {
	// Marshalers is a list of type-specific marshalers to use.
	Marshalers *Marshalers

	// TODO: Add other options.
}

// Marshal serializes a Go value as a []byte with default options.
// It is a thin wrapper over MarshalOptions.Marshal.
func Marshal(in interface{}) (out []byte, err error) {
	return MarshalOptions{}.Marshal(EncodeOptions{}, in)
}

// MarshalFull serializes a Go value into an io.Writer with default options.
// It is a thin wrapper over MarshalOptions.MarshalFull.
func MarshalFull(out io.Writer, in interface{}) error {
	return MarshalOptions{}.MarshalFull(EncodeOptions{}, out, in)
}

// Marshal serializes a Go value as a []byte according to the provided
// marshal and encode options. It does not terminate the output with a newline.
// See MarshalNext for details about the conversion of a Go value into JSON.
func (mo MarshalOptions) Marshal(eo EncodeOptions, in interface{}) (out []byte, err error) {
	panic("not implemented")
}

// MarshalFull serializes a Go value into an io.Writer according to the provided
// marshal and encode options. It does not terminate the output with a newline.
// See MarshalNext for details about the conversion of a Go value into JSON.
func (mo MarshalOptions) MarshalFull(eo EncodeOptions, out io.Writer, in interface{}) error {
	panic("not implemented")
}

// MarshalNext serializes a Go value as the next JSON value according to
// the provided marshal options.
//
// TODO: Document details for all types are marshaled.
func (mo MarshalOptions) MarshalNext(out *Encoder, in interface{}) error {
	panic("not implemented")
}

// UnmarshalOptions configures how JSON data is deserialized as Go data.
// The zero value is equivalent to the default unmarshal settings.
type UnmarshalOptions struct {
	// Unmarshalers is a list of type-specific unmarshalers to use.
	Unmarshalers *Unmarshalers

	// TODO: Add other options.
}

// Unmarshal deserializes a Go value from a []byte with default options.
// It is a thin wrapper over UnmarshalOptions.Unmarshal.
func Unmarshal(in []byte, out interface{}) error {
	return UnmarshalOptions{}.Unmarshal(DecodeOptions{}, in, out)
}

// UnmarshalFull deserializes a Go value from an io.Reader with default options.
// It is a thin wrapper over UnmarshalOptions.UnmarshalFull.
func UnmarshalFull(in io.Reader, out interface{}) error {
	return UnmarshalOptions{}.UnmarshalFull(DecodeOptions{}, in, out)
}

// Unmarshal deserializes a Go value from a []byte according to the
// provided unmarshal and decode options. The output must be a non-nil pointer.
// The input must be a single JSON value with optional whitespace interspersed.
// See UnmarshalNext for details about the conversion of JSON into a Go value.
func (uo UnmarshalOptions) Unmarshal(do DecodeOptions, in []byte, out interface{}) error {
	panic("not implemented")
}

// UnmarshalFull deserializes a Go value from an io.Reader according to the
// provided unmarshal and decode options. The output must be a non-nil pointer.
// The input must be a single JSON value with optional whitespace interspersed.
// It consumes the entirety of io.Reader until io.EOF is encountered.
// See UnmarshalNext for details about the conversion of JSON into a Go value.
func (uo UnmarshalOptions) UnmarshalFull(do DecodeOptions, in io.Reader, out interface{}) error {
	panic("not implemented")
}

// UnmarshalNext deserializes the next JSON value into a Go value according to
// the provided unmarshal options. The output must be a non-nil pointer.
//
// TODO: Document details for all types are unmarshaled.
func (uo UnmarshalOptions) UnmarshalNext(in *Decoder, out interface{}) error {
	panic("not implemented")
}
