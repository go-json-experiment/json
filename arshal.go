// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"errors"
	"io"
	"reflect"
	"sync"
)

// ErrUnknownName is a sentinel error indicating that
// an unknown name was encountered while unmarshaling a JSON object.
// It is usually wrapped within a SemanticError.
const ErrUnknownName = jsonError("unknown name")

// MarshalOptions configures how Go data is serialized as JSON data.
// The zero value is equivalent to the default marshal settings.
type MarshalOptions struct {
	requireKeyedLiterals
	nonComparable

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

	// DiscardUnknownMembers specifies that marshaling should ignore any
	// JSON object members stored in Go struct fields dedicated to storing
	// unknown JSON object members.
	DiscardUnknownMembers bool

	// formatDepth is the depth at which we respect the format flag.
	formatDepth int
	// format is custom formatting for the value at the specified depth.
	format string
}

// Marshal serializes a Go value as a []byte with default options.
// It is a thin wrapper over MarshalOptions.Marshal.
func Marshal(in any) (out []byte, err error) {
	return MarshalOptions{}.Marshal(EncodeOptions{}, in)
}

// MarshalFull serializes a Go value into an io.Writer with default options.
// It is a thin wrapper over MarshalOptions.MarshalFull.
func MarshalFull(out io.Writer, in any) error {
	return MarshalOptions{}.MarshalFull(EncodeOptions{}, out, in)
}

// Marshal serializes a Go value as a []byte according to the provided
// marshal and encode options. It does not terminate the output with a newline.
// See MarshalNext for details about the conversion of a Go value into JSON.
func (mo MarshalOptions) Marshal(eo EncodeOptions, in any) (out []byte, err error) {
	enc := getBufferedEncoder(eo)
	defer putBufferedEncoder(enc)
	enc.options.omitTopLevelNewline = true
	err = mo.MarshalNext(enc, in)
	// TODO(https://golang.org/issue/45038): Use bytes.Clone.
	return append([]byte(nil), enc.buf...), err
}

// MarshalFull serializes a Go value into an io.Writer according to the provided
// marshal and encode options. It does not terminate the output with a newline.
// See MarshalNext for details about the conversion of a Go value into JSON.
func (mo MarshalOptions) MarshalFull(eo EncodeOptions, out io.Writer, in any) error {
	// NOTE: We cannot pool the intermediate buffer since it leaks to out.
	enc := getStreamingEncoder(out, eo)
	defer putStreamingEncoder(enc)
	enc.options.omitTopLevelNewline = true
	err := mo.MarshalNext(enc, in)
	return err
}

// MarshalNext serializes a Go value as the next JSON value according to
// the provided marshal options.
//
// TODO: Document details for all types are marshaled.
func (mo MarshalOptions) MarshalNext(out *Encoder, in any) error {
	v := reflect.ValueOf(in)
	if !v.IsValid() || (v.Kind() == reflect.Pointer && v.IsNil()) {
		return out.WriteToken(Null)
	}
	// Shallow copy non-pointer values to obtain an addressable value.
	// It is beneficial to performance to always pass pointers to avoid this.
	if v.Kind() != reflect.Pointer {
		v2 := reflect.New(v.Type())
		v2.Elem().Set(v)
		v = v2
	}
	va := addressableValue{v.Elem()} // dereferenced pointer is always addressable
	t := va.Type()

	// Lookup and call the marshal function for this type.
	marshal := lookupArshaler(t).marshal
	if mo.Marshalers != nil {
		marshal, _ = mo.Marshalers.lookup(marshal, t)
	}
	if err := marshal(mo, out, va); err != nil {
		if !out.options.AllowDuplicateNames {
			out.tokens.invalidateDisabledNamespaces()
		}
		return err
	}
	return nil
}

// UnmarshalOptions configures how JSON data is deserialized as Go data.
// The zero value is equivalent to the default unmarshal settings.
type UnmarshalOptions struct {
	requireKeyedLiterals
	nonComparable

	// Unmarshalers is a list of type-specific unmarshalers to use.
	Unmarshalers *Unmarshalers

	// StringifyNumbers specifies that numeric Go types can be deserialized
	// from either a JSON number or a JSON string containing a JSON number
	// without any surrounding whitespace.
	StringifyNumbers bool

	// MatchCaseInsensitiveNames specifies that unmarshaling into a Go struct
	// should fallback on a case insensitive match of the name if an exact match
	// could not be found.
	MatchCaseInsensitiveNames bool

	// RejectUnknownNames specifies that unknown names should be rejected
	// when unmarshaling a JSON object, regardless of whether there is a field
	// to store unknown members. When an unknown name is encountered,
	// an unmarshal implementation should return an error that matches
	// ErrUnknownName according to errors.Is.
	RejectUnknownNames bool

	// formatDepth is the depth at which we respect the format flag.
	formatDepth int
	// format is custom formatting for the value at the specified depth.
	format string
}

// Unmarshal deserializes a Go value from a []byte with default options.
// It is a thin wrapper over UnmarshalOptions.Unmarshal.
func Unmarshal(in []byte, out any) error {
	return UnmarshalOptions{}.Unmarshal(DecodeOptions{}, in, out)
}

// UnmarshalFull deserializes a Go value from an io.Reader with default options.
// It is a thin wrapper over UnmarshalOptions.UnmarshalFull.
func UnmarshalFull(in io.Reader, out any) error {
	return UnmarshalOptions{}.UnmarshalFull(DecodeOptions{}, in, out)
}

// Unmarshal deserializes a Go value from a []byte according to the
// provided unmarshal and decode options. The output must be a non-nil pointer.
// The input must be a single JSON value with optional whitespace interspersed.
// See UnmarshalNext for details about the conversion of JSON into a Go value.
func (uo UnmarshalOptions) Unmarshal(do DecodeOptions, in []byte, out any) error {
	dec := getBufferedDecoder(in, do)
	defer putBufferedDecoder(dec)
	return uo.unmarshalFull(dec, out)
}

// UnmarshalFull deserializes a Go value from an io.Reader according to the
// provided unmarshal and decode options. The output must be a non-nil pointer.
// The input must be a single JSON value with optional whitespace interspersed.
// It consumes the entirety of io.Reader until io.EOF is encountered.
// See UnmarshalNext for details about the conversion of JSON into a Go value.
func (uo UnmarshalOptions) UnmarshalFull(do DecodeOptions, in io.Reader, out any) error {
	// NOTE: We cannot pool the intermediate buffer since it leaks to in.
	dec := getStreamingDecoder(in, do)
	defer putStreamingDecoder(dec)
	return uo.unmarshalFull(dec, out)
}
func (uo UnmarshalOptions) unmarshalFull(in *Decoder, out any) error {
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
func (uo UnmarshalOptions) UnmarshalNext(in *Decoder, out any) error {
	v := reflect.ValueOf(out)
	if !v.IsValid() || v.Kind() != reflect.Pointer || v.IsNil() {
		var t reflect.Type
		if v.IsValid() {
			t = v.Type()
			if t.Kind() == reflect.Pointer {
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
	if uo.Unmarshalers != nil {
		unmarshal, _ = uo.Unmarshalers.lookup(unmarshal, t)
	}
	if err := unmarshal(uo, in, va); err != nil {
		if !in.options.AllowDuplicateNames {
			in.tokens.invalidateDisabledNamespaces()
		}
		return err
	}
	return nil
}

// addressableValue is a reflect.Value that is guaranteed to be addressable
// such that calling the Addr and Set methods do not panic.
//
// There is no compile magic that enforces this property,
// but rather the need to construct this type makes it easier to examine each
// construction site to ensure that this property is upheld.
type addressableValue struct{ reflect.Value }

// newAddressableValue constructs a new addressable value of type t.
func newAddressableValue(t reflect.Type) addressableValue {
	return addressableValue{reflect.New(t).Elem()}
}

// All marshal and unmarshal behavior is implemented using these signatures.
type (
	marshaler   = func(MarshalOptions, *Encoder, addressableValue) error
	unmarshaler = func(UnmarshalOptions, *Decoder, addressableValue) error
)

type arshaler struct {
	marshal    marshaler
	unmarshal  unmarshaler
	nonDefault bool
}

var lookupArshalerCache sync.Map // map[reflect.Type]*arshaler

func lookupArshaler(t reflect.Type) *arshaler {
	if v, ok := lookupArshalerCache.Load(t); ok {
		return v.(*arshaler)
	}

	fncs := makeDefaultArshaler(t)
	fncs = makeMethodArshaler(fncs, t)
	fncs = makeTimeArshaler(fncs, t)

	// Use the last stored so that duplicate arshalers can be garbage collected.
	v, _ := lookupArshalerCache.LoadOrStore(t, fncs)
	return v.(*arshaler)
}
