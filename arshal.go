// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"errors"
	"io"
	"reflect"
)

// MarshalOptions configures how Go data is serialized as JSON data.
// The zero value is equivalent to the default marshal settings.
type MarshalOptions struct {
	// Marshalers is a list of type-specific marshalers to use.
	Marshalers *Marshalers

	// StringifyNumbers specifies that numeric Go types should be serialized
	// as a JSON string containing the equivalent JSON number value.
	//
	// According to RFC 8259, section 6, a JSON implementation may choose to
	// limit the representation of a JSON number to an IEEE 754 binary64 value.
	// This may cause decoders to lose precision for int64 and uint64 types.
	// Escaping JSON numbers as a JSON string preserves the exact precision.
	StringifyNumbers bool

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
	enc := eo.newEncoder(nil, nil) // TODO: Pool this.
	enc.options.omitTopLevelNewline = true
	err = mo.MarshalNext(enc, in)
	return enc.buf, err
}

// MarshalFull serializes a Go value into an io.Writer according to the provided
// marshal and encode options. It does not terminate the output with a newline.
// See MarshalNext for details about the conversion of a Go value into JSON.
func (mo MarshalOptions) MarshalFull(eo EncodeOptions, out io.Writer, in interface{}) error {
	enc := eo.NewEncoder(out) // TODO: Pool this.
	enc.options.omitTopLevelNewline = true
	err := mo.MarshalNext(enc, in)
	return err
}

// MarshalNext serializes a Go value as the next JSON value according to
// the provided marshal options.
//
// TODO: Document details for all types are marshaled.
func (mo MarshalOptions) MarshalNext(out *Encoder, in interface{}) error {
	v := reflect.ValueOf(in)
	if !v.IsValid() || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return out.WriteToken(Null)
	}
	// Shallow copy non-pointer values to obtain an addressable value.
	// It is beneficial to performance to always pass ponters to avoid this.
	if v.Kind() != reflect.Ptr {
		v2 := reflect.New(v.Type())
		v2.Elem().Set(v)
		v = v2
	}
	va := addressableValue{v.Elem()} // dereferenced pointer is always addressable
	t := va.Type()

	// Lookup and call the marshal function for this type.
	marshal := lookupArshaler(t).marshal
	// TODO: Handle custom marshalers.
	return marshal(mo, out, va)
}

// UnmarshalOptions configures how JSON data is deserialized as Go data.
// The zero value is equivalent to the default unmarshal settings.
type UnmarshalOptions struct {
	// Unmarshalers is a list of type-specific unmarshalers to use.
	Unmarshalers *Unmarshalers

	// StringifyNumbers specifies that numeric Go types can be deserialized
	// from either a JSON number or a JSON string containing a JSON number.
	StringifyNumbers bool

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
	dec := do.newDecoder(nil, in) // TODO: Pool this.
	return uo.unmarshalFull(dec, out)
}

// UnmarshalFull deserializes a Go value from an io.Reader according to the
// provided unmarshal and decode options. The output must be a non-nil pointer.
// The input must be a single JSON value with optional whitespace interspersed.
// It consumes the entirety of io.Reader until io.EOF is encountered.
// See UnmarshalNext for details about the conversion of JSON into a Go value.
func (uo UnmarshalOptions) UnmarshalFull(do DecodeOptions, in io.Reader, out interface{}) error {
	// NOTE: We cannot pool the intermediate buffer since it leaks to in.
	dec := do.NewDecoder(in)
	return uo.unmarshalFull(dec, out)
}
func (uo UnmarshalOptions) unmarshalFull(in *Decoder, out interface{}) error {
	switch err := uo.UnmarshalNext(in, out); err {
	case nil:
		return in.checkEOF()
	case io.EOF:
		return io.ErrUnexpectedEOF
	default:
		return err
	}
}

// UnmarshalNext deserializes the next JSON value into a Go value according to
// the provided unmarshal options. The output must be a non-nil pointer.
//
// TODO: Document details for all types are unmarshaled.
func (uo UnmarshalOptions) UnmarshalNext(in *Decoder, out interface{}) error {
	v := reflect.ValueOf(out)
	if !v.IsValid() || (v.Kind() != reflect.Ptr || v.IsNil()) {
		var t reflect.Type
		if v.IsValid() {
			t = v.Type()
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}
		}
		err := errors.New("value must be passed as a non-nil pointer reference")
		return &SemanticError{action: "unmarshal", GoType: t, Err: err}
	}
	va := addressableValue{v.Elem()} // dereferenced pointer is always addressable
	t := va.Type()

	// Lookup and call the unmarshal function for this type.
	unmarshal := lookupArshaler(t).unmarshal
	// TODO: Handle custom unmarshalers.
	return unmarshal(uo, in, va)
}
