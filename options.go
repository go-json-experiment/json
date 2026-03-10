// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !goexperiment.jsonv2 || !go1.25

package json

import (
	"fmt"

	"github.com/go-json-experiment/json/internal/jsonflags"
	"github.com/go-json-experiment/json/internal/jsonopts"
)

// TODO: We should remove the Option alias and just use jsonots.Option directly.
// Ideally, this entire file is a GoDoc section (https://go.dev/issue/44447).

// Option configure [Marshal], [MarshalWrite], [MarshalEncode],
// [Unmarshal], [UnmarshalRead], and [UnmarshalDecode] with specific features.
// Each function takes in a variadic list of options, where properties
// set in later options override the value of previously set properties.
//
// There is a single Option type, which is used with both marshal and unmarshal.
// Some options affect both operations, while others only affect one operation:
//
//   - [StringifyNumbers] affects marshaling and unmarshaling
//   - [Deterministic] affects marshaling only
//   - [FormatNilSliceAsNull] affects marshaling only
//   - [FormatNilMapAsNull] affects marshaling only
//   - [OmitZeroStructFields] affects marshaling only
//   - [MatchCaseInsensitiveNames] affects marshaling and unmarshaling
//   - [RejectUnknownMembers] affects unmarshaling only
//   - [Marshalers] affects marshaling only
//   - [Unmarshalers] affects unmarshaling only
//
// Options that do not affect a particular operation are ignored.
//
// The Option type is identical to [encoding/json.Option] and
// [encoding/json/jsontext.Option]. Options from the other packages can
// be used interchangeably with functionality in this package.
type Option = jsonopts.Option

// DefaultOptionsV2 is the full set of all options that define v2 semantics.
// It is equivalent to the set of options in [encoding/json.DefaultOptionsV1]
// all being set to false. All other options are not present.
func DefaultOptionsV2() jsonopts.Options {
	return jsonopts.DefaultOptionsV2.AsOptions()
}

// StringifyNumbers is a [bool] option that specifies whether
// numeric Go types should be marshaled as a JSON string
// containing the equivalent JSON number value.
// When unmarshaling, numeric Go types are parsed from a JSON string
// containing the JSON number without any surrounding whitespace.
//
// According to RFC 8259, section 6, a JSON implementation may choose to
// limit the representation of a JSON number to an IEEE 754 binary64 value.
// This may cause decoders to lose precision for int64 and uint64 types.
// Quoting JSON numbers as a JSON string preserves the exact precision.
//
// This affects either marshaling or unmarshaling.
type StringifyNumbers = jsonopts.StringifyNumbers

// Deterministic is a [bool] option that specifies whether
// the same input value will be serialized as the exact same output bytes.
// Different processes of the same program will serialize equal values to the same bytes,
// but different versions of the same program are not guaranteed
// to produce the exact same sequence of bytes.
//
// This only affects marshaling and is ignored when unmarshaling.
type Deterministic = jsonopts.Deterministic

// FormatNilSliceAsNull is a [bool] option that specifies whether
// a nil Go slice should marshal as a JSON null instead of
// the default representation as an empty JSON array
// (or an empty JSON string in the case of ~[]byte).
// Slice fields explicitly marked with `format:emitempty` still marshal
// as an empty JSON array.
//
// This only affects marshaling and is ignored when unmarshaling.
type FormatNilSliceAsNull = jsonopts.FormatNilSliceAsNull

// FormatNilMapAsNull is a [bool] option that specifies whether
// a nil Go map should marshal as a JSON null instead of
// the default representation as an empty JSON object.
// Map fields explicitly marked with `format:emitempty` still marshal
// as an empty JSON object.
//
// This only affects marshaling and is ignored when unmarshaling.
type FormatNilMapAsNull = jsonopts.FormatNilMapAsNull

// OmitZeroStructFields is a [bool] option that specifies whether
// a Go struct should marshal in such a way that all struct fields
// are omitted from the marshaled output if the value is zero
// as determined by the "IsZero() bool" method if present,
// otherwise based on whether the field is the zero Go value.
// This is semantically equivalent to specifying the `omitzero` tag option
// on every field in a Go struct.
//
// This only affects marshaling and is ignored when unmarshaling.
type OmitZeroStructFields = jsonopts.OmitZeroStructFields

// MatchCaseInsensitiveNames is a [bool] option that specifies
// whether JSON object members are matched against Go struct fields
// using a case-insensitive match of the name.
// Go struct fields explicitly marked with `case:strict` or `case:ignore`
// always use case-sensitive (or case-insensitive) name matching,
// regardless of the value of this option.
//
// This affects either marshaling or unmarshaling.
// For marshaling, this option may alter the detection of duplicate names
// (assuming [jsontext.AllowDuplicateNames] is false) from inlined fields
// if it matches one of the declared fields in the Go struct.
type MatchCaseInsensitiveNames = jsonopts.MatchCaseInsensitiveNames

// RejectUnknownMembers is a [bool] option that specifies
// whether unknown members should be rejected when unmarshaling a JSON object.
//
// This only affects unmarshaling and is ignored when marshaling.
type RejectUnknownMembers = jsonopts.RejectUnknownMembers

// TODO: Do we need an opaque WithMarshalers or WithUnmarshals type
// just to implement Option?
// Right now it is easy to accidentally do:
//	json.Marshal(v, json.MarshalFunc(...), json.MarshalFunc(...))
// without realizing that you should actually do:
//	json.Marshal(v, json.JoinMarshalers(json.MarshalFunc(...), json.MarshalFunc(...)))
// However, we can have Go vet detect and reject that.

// Inject support into "jsonopts" to handle these types.
func init() {
	jsonopts.GetUnknownOption = func(src jsonopts.Struct, zero jsonopts.Option) (any, bool) {
		switch zero.(type) {
		case *Marshalers:
			if !src.Flags.Has(jsonflags.Marshalers) {
				return (*Marshalers)(nil), false
			}
			return src.Marshalers.(*Marshalers), true
		case *Unmarshalers:
			if !src.Flags.Has(jsonflags.Unmarshalers) {
				return (*Unmarshalers)(nil), false
			}
			return src.Unmarshalers.(*Unmarshalers), true
		default:
			panic(fmt.Sprintf("unknown option %T", zero))
		}
	}
	jsonopts.JoinUnknownOption = func(dst jsonopts.Struct, src jsonopts.Option) jsonopts.Struct {
		switch src := src.(type) {
		case *Marshalers:
			dst.Flags.Set(jsonflags.Marshalers | 1)
			dst.Marshalers = (*Marshalers)(src)
		case *Unmarshalers:
			dst.Flags.Set(jsonflags.Unmarshalers | 1)
			dst.Unmarshalers = (*Unmarshalers)(src)
		default:
			panic(fmt.Sprintf("unknown option %T", src))
		}
		return dst
	}
}
