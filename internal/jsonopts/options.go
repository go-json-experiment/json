// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !goexperiment.jsonv2 || !go1.25

package jsonopts

import (
	"github.com/go-json-experiment/json/internal/jsonflags"
)

type Option interface {
	// TODO(https://go.dev/issue/71664): Remove this? This would allow any arbitrary user option type.
	isJSONOption()
}

type Options struct{ s Struct }

func (Options) isJSONOption() {}

// OptionMarker is a zero-sized concrete type that implements [Option].
type OptionMarker struct{}

func (OptionMarker) isJSONOption() {}

// Struct is the combination of all options in struct form.
// This is efficient to pass down the call stack and to query.
type Struct struct {
	Flags jsonflags.Flags

	CoderValues
	ArshalValues
}

func (s Struct) AsOptions() Options { return Options{s} }

func AsStruct(o Options) Struct { return o.s }

type CoderValues struct {
	Indent       string // jsonflags.Indent
	IndentPrefix string // jsonflags.IndentPrefix
	ByteLimit    int64  // jsonflags.ByteLimit
	DepthLimit   int    // jsonflags.DepthLimit
}

type ArshalValues struct {
	// The Marshalers and Unmarshalers fields use the any type to avoid a
	// concrete dependency on *json.Marshalers and *json.Unmarshalers,
	// which would in turn create a dependency on the "reflect" package.

	Marshalers   any // jsonflags.Marshalers
	Unmarshalers any // jsonflags.Unmarshalers

	Format      string
	FormatDepth int
}

// DefaultOptionsV2 is the set of all options that define default v2 behavior.
var DefaultOptionsV2 = Struct{
	Flags: jsonflags.Flags{
		Presence: uint64(jsonflags.DefaultV1Flags),
		Values:   uint64(0), // all flags in DefaultV1Flags are false
	},
}

// DefaultOptionsV1 is the set of all options that define default v1 behavior.
var DefaultOptionsV1 = Struct{
	Flags: jsonflags.Flags{
		Presence: uint64(jsonflags.DefaultV1Flags),
		Values:   uint64(jsonflags.DefaultV1Flags), // all flags in DefaultV1Flags are true
	},
}

// GetUnknownOption is injected by the "json" package to handle Options
// declared in that package so that "jsonopts" can handle them.
var GetUnknownOption = func(_ Struct, zero Option) (any, bool) { return zero, false }

func GetOption[T Option](structOpts Options) (T, bool) {
	// Lookup the option based on the return value of the setter.
	var zero T
	switch any(zero).(type) {
	case AllowDuplicateNames:
		v := structOpts.s.Flags.Get(jsonflags.AllowDuplicateNames)
		ok := structOpts.s.Flags.Has(jsonflags.AllowDuplicateNames)
		return any(AllowDuplicateNames(v)).(T), ok
	case AllowInvalidUTF8:
		v := structOpts.s.Flags.Get(jsonflags.AllowInvalidUTF8)
		ok := structOpts.s.Flags.Has(jsonflags.AllowInvalidUTF8)
		return any(AllowInvalidUTF8(v)).(T), ok
	case OmitTopLevelNewline:
		v := structOpts.s.Flags.Get(jsonflags.OmitTopLevelNewline)
		ok := structOpts.s.Flags.Has(jsonflags.OmitTopLevelNewline)
		return any(OmitTopLevelNewline(v)).(T), ok
	case EscapeForHTML:
		v := structOpts.s.Flags.Get(jsonflags.EscapeForHTML)
		ok := structOpts.s.Flags.Has(jsonflags.EscapeForHTML)
		return any(EscapeForHTML(v)).(T), ok
	case EscapeForJS:
		v := structOpts.s.Flags.Get(jsonflags.EscapeForJS)
		ok := structOpts.s.Flags.Has(jsonflags.EscapeForJS)
		return any(EscapeForJS(v)).(T), ok
	case PreserveRawStrings:
		v := structOpts.s.Flags.Get(jsonflags.PreserveRawStrings)
		ok := structOpts.s.Flags.Has(jsonflags.PreserveRawStrings)
		return any(PreserveRawStrings(v)).(T), ok
	case CanonicalizeRawInts:
		v := structOpts.s.Flags.Get(jsonflags.CanonicalizeRawInts)
		ok := structOpts.s.Flags.Has(jsonflags.CanonicalizeRawInts)
		return any(CanonicalizeRawInts(v)).(T), ok
	case CanonicalizeRawFloats:
		v := structOpts.s.Flags.Get(jsonflags.CanonicalizeRawFloats)
		ok := structOpts.s.Flags.Has(jsonflags.CanonicalizeRawFloats)
		return any(CanonicalizeRawFloats(v)).(T), ok
	case ReorderRawObjects:
		v := structOpts.s.Flags.Get(jsonflags.ReorderRawObjects)
		ok := structOpts.s.Flags.Has(jsonflags.ReorderRawObjects)
		return any(ReorderRawObjects(v)).(T), ok
	case SpaceAfterColon:
		v := structOpts.s.Flags.Get(jsonflags.SpaceAfterColon)
		ok := structOpts.s.Flags.Has(jsonflags.SpaceAfterColon)
		return any(SpaceAfterColon(v)).(T), ok
	case SpaceAfterComma:
		v := structOpts.s.Flags.Get(jsonflags.SpaceAfterComma)
		ok := structOpts.s.Flags.Has(jsonflags.SpaceAfterComma)
		return any(SpaceAfterComma(v)).(T), ok
	case Multiline:
		v := structOpts.s.Flags.Get(jsonflags.Multiline)
		ok := structOpts.s.Flags.Has(jsonflags.Multiline)
		return any(Multiline(v)).(T), ok
	case StringifyNumbers:
		v := structOpts.s.Flags.Get(jsonflags.StringifyNumbers)
		ok := structOpts.s.Flags.Has(jsonflags.StringifyNumbers)
		return any(StringifyNumbers(v)).(T), ok
	case Deterministic:
		v := structOpts.s.Flags.Get(jsonflags.Deterministic)
		ok := structOpts.s.Flags.Has(jsonflags.Deterministic)
		return any(Deterministic(v)).(T), ok
	case FormatNilMapAsNull:
		v := structOpts.s.Flags.Get(jsonflags.FormatNilMapAsNull)
		ok := structOpts.s.Flags.Has(jsonflags.FormatNilMapAsNull)
		return any(FormatNilMapAsNull(v)).(T), ok
	case FormatNilSliceAsNull:
		v := structOpts.s.Flags.Get(jsonflags.FormatNilSliceAsNull)
		ok := structOpts.s.Flags.Has(jsonflags.FormatNilSliceAsNull)
		return any(FormatNilSliceAsNull(v)).(T), ok
	case OmitZeroStructFields:
		v := structOpts.s.Flags.Get(jsonflags.OmitZeroStructFields)
		ok := structOpts.s.Flags.Has(jsonflags.OmitZeroStructFields)
		return any(OmitZeroStructFields(v)).(T), ok
	case MatchCaseInsensitiveNames:
		v := structOpts.s.Flags.Get(jsonflags.MatchCaseInsensitiveNames)
		ok := structOpts.s.Flags.Has(jsonflags.MatchCaseInsensitiveNames)
		return any(MatchCaseInsensitiveNames(v)).(T), ok
	case RejectUnknownMembers:
		v := structOpts.s.Flags.Get(jsonflags.RejectUnknownMembers)
		ok := structOpts.s.Flags.Has(jsonflags.RejectUnknownMembers)
		return any(RejectUnknownMembers(v)).(T), ok
	case CallMethodsWithLegacySemantics:
		v := structOpts.s.Flags.Get(jsonflags.CallMethodsWithLegacySemantics)
		ok := structOpts.s.Flags.Has(jsonflags.CallMethodsWithLegacySemantics)
		return any(CallMethodsWithLegacySemantics(v)).(T), ok
	case FormatByteArrayAsArray:
		v := structOpts.s.Flags.Get(jsonflags.FormatByteArrayAsArray)
		ok := structOpts.s.Flags.Has(jsonflags.FormatByteArrayAsArray)
		return any(FormatByteArrayAsArray(v)).(T), ok
	case FormatBytesWithLegacySemantics:
		v := structOpts.s.Flags.Get(jsonflags.FormatBytesWithLegacySemantics)
		ok := structOpts.s.Flags.Has(jsonflags.FormatBytesWithLegacySemantics)
		return any(FormatBytesWithLegacySemantics(v)).(T), ok
	case FormatDurationAsNano:
		v := structOpts.s.Flags.Get(jsonflags.FormatDurationAsNano)
		ok := structOpts.s.Flags.Has(jsonflags.FormatDurationAsNano)
		return any(FormatDurationAsNano(v)).(T), ok
	case MatchCaseSensitiveDelimiter:
		v := structOpts.s.Flags.Get(jsonflags.MatchCaseSensitiveDelimiter)
		ok := structOpts.s.Flags.Has(jsonflags.MatchCaseSensitiveDelimiter)
		return any(MatchCaseSensitiveDelimiter(v)).(T), ok
	case MergeWithLegacySemantics:
		v := structOpts.s.Flags.Get(jsonflags.MergeWithLegacySemantics)
		ok := structOpts.s.Flags.Has(jsonflags.MergeWithLegacySemantics)
		return any(MergeWithLegacySemantics(v)).(T), ok
	case OmitEmptyWithLegacySemantics:
		v := structOpts.s.Flags.Get(jsonflags.OmitEmptyWithLegacySemantics)
		ok := structOpts.s.Flags.Has(jsonflags.OmitEmptyWithLegacySemantics)
		return any(OmitEmptyWithLegacySemantics(v)).(T), ok
	case ParseBytesWithLooseRFC4648:
		v := structOpts.s.Flags.Get(jsonflags.ParseBytesWithLooseRFC4648)
		ok := structOpts.s.Flags.Has(jsonflags.ParseBytesWithLooseRFC4648)
		return any(ParseBytesWithLooseRFC4648(v)).(T), ok
	case ParseTimeWithLooseRFC3339:
		v := structOpts.s.Flags.Get(jsonflags.ParseTimeWithLooseRFC3339)
		ok := structOpts.s.Flags.Has(jsonflags.ParseTimeWithLooseRFC3339)
		return any(ParseTimeWithLooseRFC3339(v)).(T), ok
	case ReportErrorsWithLegacySemantics:
		v := structOpts.s.Flags.Get(jsonflags.ReportErrorsWithLegacySemantics)
		ok := structOpts.s.Flags.Has(jsonflags.ReportErrorsWithLegacySemantics)
		return any(ReportErrorsWithLegacySemantics(v)).(T), ok
	case StringifyWithLegacySemantics:
		v := structOpts.s.Flags.Get(jsonflags.StringifyWithLegacySemantics)
		ok := structOpts.s.Flags.Has(jsonflags.StringifyWithLegacySemantics)
		return any(StringifyWithLegacySemantics(v)).(T), ok
	case StringifyBoolsAndStrings:
		v := structOpts.s.Flags.Get(jsonflags.StringifyBoolsAndStrings)
		ok := structOpts.s.Flags.Has(jsonflags.StringifyBoolsAndStrings)
		return any(StringifyBoolsAndStrings(v)).(T), ok
	case UnmarshalAnyWithRawNumber:
		v := structOpts.s.Flags.Get(jsonflags.UnmarshalAnyWithRawNumber)
		ok := structOpts.s.Flags.Has(jsonflags.UnmarshalAnyWithRawNumber)
		return any(UnmarshalAnyWithRawNumber(v)).(T), ok
	case UnmarshalArrayFromAnyLength:
		v := structOpts.s.Flags.Get(jsonflags.UnmarshalArrayFromAnyLength)
		ok := structOpts.s.Flags.Has(jsonflags.UnmarshalArrayFromAnyLength)
		return any(UnmarshalArrayFromAnyLength(v)).(T), ok
	case Indent:
		if !structOpts.s.Flags.Has(jsonflags.Indent) {
			return zero, false
		}
		return any(Indent(structOpts.s.Indent)).(T), true
	case IndentPrefix:
		if !structOpts.s.Flags.Has(jsonflags.IndentPrefix) {
			return zero, false
		}
		return any(IndentPrefix(structOpts.s.IndentPrefix)).(T), true
	case ByteLimit:
		if !structOpts.s.Flags.Has(jsonflags.ByteLimit) {
			return zero, false
		}
		return any(ByteLimit(structOpts.s.ByteLimit)).(T), true
	case DepthLimit:
		if !structOpts.s.Flags.Has(jsonflags.DepthLimit) {
			return zero, false
		}
		return any(DepthLimit(structOpts.s.DepthLimit)).(T), true
	default:
		v, ok := GetUnknownOption(structOpts.s, zero)
		return v.(T), ok
	}
}

func JoinOptions(opts ...Option) Options {
	var s Struct
	s.Join(opts...)
	return s.AsOptions()
}

// JoinUnknownOption is injected by the "json" package to handle Option
// declared in that package so that "jsonopts" can handle them.
var JoinUnknownOption = func(s Struct, _ Option) Struct { return s }

func (dst *Struct) Join(srcs ...Option) {
	dst.join(false, srcs...)
}

func (dst *Struct) JoinWithoutCoderOptions(srcs ...Option) {
	dst.join(true, srcs...)
}

func (dst *Struct) join(excludeCoderOptions bool, srcs ...Option) {
	for _, src := range srcs {
		switch src := src.(type) {
		case nil:
			continue
		case AllowDuplicateNames:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.AllowDuplicateNames | jsonflags.Bool(bool(src)))
		case AllowInvalidUTF8:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.AllowInvalidUTF8 | jsonflags.Bool(bool(src)))
		case OmitTopLevelNewline:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.OmitTopLevelNewline | jsonflags.Bool(bool(src)))
		case EscapeForHTML:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.EscapeForHTML | jsonflags.Bool(bool(src)))
		case EscapeForJS:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.EscapeForJS | jsonflags.Bool(bool(src)))
		case PreserveRawStrings:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.PreserveRawStrings | jsonflags.Bool(bool(src)))
		case CanonicalizeRawInts:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.CanonicalizeRawInts | jsonflags.Bool(bool(src)))
		case CanonicalizeRawFloats:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.CanonicalizeRawFloats | jsonflags.Bool(bool(src)))
		case ReorderRawObjects:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.ReorderRawObjects | jsonflags.Bool(bool(src)))
		case SpaceAfterColon:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.SpaceAfterColon | jsonflags.Bool(bool(src)))
		case SpaceAfterComma:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.SpaceAfterComma | jsonflags.Bool(bool(src)))
		case Multiline:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.Multiline | jsonflags.Bool(bool(src)))
		case StringifyNumbers:
			dst.Flags.Set(jsonflags.StringifyNumbers | jsonflags.Bool(bool(src)))
		case Deterministic:
			dst.Flags.Set(jsonflags.Deterministic | jsonflags.Bool(bool(src)))
		case FormatNilMapAsNull:
			dst.Flags.Set(jsonflags.FormatNilMapAsNull | jsonflags.Bool(bool(src)))
		case FormatNilSliceAsNull:
			dst.Flags.Set(jsonflags.FormatNilSliceAsNull | jsonflags.Bool(bool(src)))
		case OmitZeroStructFields:
			dst.Flags.Set(jsonflags.OmitZeroStructFields | jsonflags.Bool(bool(src)))
		case MatchCaseInsensitiveNames:
			dst.Flags.Set(jsonflags.MatchCaseInsensitiveNames | jsonflags.Bool(bool(src)))
		case RejectUnknownMembers:
			dst.Flags.Set(jsonflags.RejectUnknownMembers | jsonflags.Bool(bool(src)))
		case CallMethodsWithLegacySemantics:
			dst.Flags.Set(jsonflags.CallMethodsWithLegacySemantics | jsonflags.Bool(bool(src)))
		case FormatByteArrayAsArray:
			dst.Flags.Set(jsonflags.FormatByteArrayAsArray | jsonflags.Bool(bool(src)))
		case FormatBytesWithLegacySemantics:
			dst.Flags.Set(jsonflags.FormatBytesWithLegacySemantics | jsonflags.Bool(bool(src)))
		case FormatDurationAsNano:
			dst.Flags.Set(jsonflags.FormatDurationAsNano | jsonflags.Bool(bool(src)))
		case MatchCaseSensitiveDelimiter:
			dst.Flags.Set(jsonflags.MatchCaseSensitiveDelimiter | jsonflags.Bool(bool(src)))
		case MergeWithLegacySemantics:
			dst.Flags.Set(jsonflags.MergeWithLegacySemantics | jsonflags.Bool(bool(src)))
		case OmitEmptyWithLegacySemantics:
			dst.Flags.Set(jsonflags.OmitEmptyWithLegacySemantics | jsonflags.Bool(bool(src)))
		case ParseBytesWithLooseRFC4648:
			dst.Flags.Set(jsonflags.ParseBytesWithLooseRFC4648 | jsonflags.Bool(bool(src)))
		case ParseTimeWithLooseRFC3339:
			dst.Flags.Set(jsonflags.ParseTimeWithLooseRFC3339 | jsonflags.Bool(bool(src)))
		case ReportErrorsWithLegacySemantics:
			dst.Flags.Set(jsonflags.ReportErrorsWithLegacySemantics | jsonflags.Bool(bool(src)))
		case StringifyWithLegacySemantics:
			dst.Flags.Set(jsonflags.StringifyWithLegacySemantics | jsonflags.Bool(bool(src)))
		case StringifyBoolsAndStrings:
			dst.Flags.Set(jsonflags.StringifyBoolsAndStrings | jsonflags.Bool(bool(src)))
		case UnmarshalAnyWithRawNumber:
			dst.Flags.Set(jsonflags.UnmarshalAnyWithRawNumber | jsonflags.Bool(bool(src)))
		case UnmarshalArrayFromAnyLength:
			dst.Flags.Set(jsonflags.UnmarshalArrayFromAnyLength | jsonflags.Bool(bool(src)))
		case Indent:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.Multiline | jsonflags.Indent | 1)
			dst.Indent = string(src)
		case IndentPrefix:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.Multiline | jsonflags.IndentPrefix | 1)
			dst.IndentPrefix = string(src)
		case ByteLimit:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.ByteLimit | 1)
			dst.ByteLimit = int64(src)
		case DepthLimit:
			if excludeCoderOptions {
				continue
			}
			dst.Flags.Set(jsonflags.DepthLimit | 1)
			dst.DepthLimit = int(src)
		case Options:
			srcFlags := src.s.Flags // shallow copy the flags
			if excludeCoderOptions {
				srcFlags.Clear(jsonflags.AllCoderFlags)
			}
			dst.Flags.Join(srcFlags)
			if srcFlags.Has(jsonflags.NonBooleanFlags) {
				if srcFlags.Has(jsonflags.Indent) {
					dst.Indent = src.s.Indent
				}
				if srcFlags.Has(jsonflags.IndentPrefix) {
					dst.IndentPrefix = src.s.IndentPrefix
				}
				if srcFlags.Has(jsonflags.ByteLimit) {
					dst.ByteLimit = src.s.ByteLimit
				}
				if srcFlags.Has(jsonflags.DepthLimit) {
					dst.DepthLimit = src.s.DepthLimit
				}
				if srcFlags.Has(jsonflags.Marshalers) {
					dst.Marshalers = src.s.Marshalers
				}
				if srcFlags.Has(jsonflags.Unmarshalers) {
					dst.Unmarshalers = src.s.Unmarshalers
				}
			}
		default:
			*dst = JoinUnknownOption(*dst, src)
		}
	}
}

// Encoder and decoder options.
type (
	AllowDuplicateNames   bool
	AllowInvalidUTF8      bool
	OmitTopLevelNewline   bool
	EscapeForHTML         bool
	EscapeForJS           bool
	PreserveRawStrings    bool
	CanonicalizeRawInts   bool
	CanonicalizeRawFloats bool
	ReorderRawObjects     bool
	SpaceAfterColon       bool
	SpaceAfterComma       bool
	Multiline             bool
	Indent                string
	IndentPrefix          string
	ByteLimit             int64
	DepthLimit            int
)

func (AllowDuplicateNames) isJSONOption()   {}
func (AllowInvalidUTF8) isJSONOption()      {}
func (OmitTopLevelNewline) isJSONOption()   {}
func (EscapeForHTML) isJSONOption()         {}
func (EscapeForJS) isJSONOption()           {}
func (PreserveRawStrings) isJSONOption()    {}
func (CanonicalizeRawInts) isJSONOption()   {}
func (CanonicalizeRawFloats) isJSONOption() {}
func (ReorderRawObjects) isJSONOption()     {}
func (SpaceAfterColon) isJSONOption()       {}
func (SpaceAfterComma) isJSONOption()       {}
func (Multiline) isJSONOption()             {}
func (Indent) isJSONOption()                {}
func (IndentPrefix) isJSONOption()          {}
func (ByteLimit) isJSONOption()             {}
func (DepthLimit) isJSONOption()            {}

// Marshal and Unmarshal options (for v2).
type (
	StringifyNumbers          bool
	Deterministic             bool
	FormatNilSliceAsNull      bool
	FormatNilMapAsNull        bool
	OmitZeroStructFields      bool
	MatchCaseInsensitiveNames bool
	RejectUnknownMembers      bool
	// type for jsonflags.Marshalers declared in "json" package
	// type for jsonflags.Unmarshalers declared in "json" package
)

func (StringifyNumbers) isJSONOption()          {}
func (Deterministic) isJSONOption()             {}
func (FormatNilMapAsNull) isJSONOption()        {}
func (FormatNilSliceAsNull) isJSONOption()      {}
func (OmitZeroStructFields) isJSONOption()      {}
func (MatchCaseInsensitiveNames) isJSONOption() {}
func (RejectUnknownMembers) isJSONOption()      {}

// Marshal and Unmarshal options (for v1).
type (
	CallMethodsWithLegacySemantics  bool
	FormatByteArrayAsArray          bool
	FormatBytesWithLegacySemantics  bool
	FormatDurationAsNano            bool
	MatchCaseSensitiveDelimiter     bool
	MergeWithLegacySemantics        bool
	OmitEmptyWithLegacySemantics    bool
	ParseBytesWithLooseRFC4648      bool
	ParseTimeWithLooseRFC3339       bool
	ReportErrorsWithLegacySemantics bool
	StringifyWithLegacySemantics    bool
	StringifyBoolsAndStrings        bool
	UnmarshalAnyWithRawNumber       bool
	UnmarshalArrayFromAnyLength     bool
)

func (CallMethodsWithLegacySemantics) isJSONOption()  {}
func (FormatByteArrayAsArray) isJSONOption()          {}
func (FormatBytesWithLegacySemantics) isJSONOption()  {}
func (FormatDurationAsNano) isJSONOption()            {}
func (MatchCaseSensitiveDelimiter) isJSONOption()     {}
func (MergeWithLegacySemantics) isJSONOption()        {}
func (OmitEmptyWithLegacySemantics) isJSONOption()    {}
func (ParseBytesWithLooseRFC4648) isJSONOption()      {}
func (ParseTimeWithLooseRFC3339) isJSONOption()       {}
func (ReportErrorsWithLegacySemantics) isJSONOption() {}
func (StringifyWithLegacySemantics) isJSONOption()    {}
func (StringifyBoolsAndStrings) isJSONOption()        {}
func (UnmarshalAnyWithRawNumber) isJSONOption()       {}
func (UnmarshalArrayFromAnyLength) isJSONOption()     {}
