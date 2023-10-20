package json

import (
	"github.com/go-json-experiment/json/internal"
	"github.com/go-json-experiment/json/internal/jsonflags"
	"github.com/go-json-experiment/json/internal/jsonopts"
)

// OptionsArshaler defines a method for allowing `json` to set options from
// `jsonopts` without creating an import cycle.
type OptionsArshaler interface {
	OptionsArshal(*jsonopts.Struct, internal.NotForPublicUse)
}

// OptionsCoder defines a method for allowing `json` to set options from
// `jsonopts` without creating an import cycle.
type OptionsCoder interface {
	OptionsCode(*jsonopts.Struct, internal.NotForPublicUse)
}

var _ OptionsArshaler = (*MarshalOptions)(nil)
var _ jsonopts.Options = (*MarshalOptions)(nil)

type MarshalOptions struct {
	StringifyNumbers          bool
	Deterministic             bool
	FormatNilSliceAsNull      bool
	FormatNilMapAsNull        bool
	MatchCaseInsensitiveNames bool
	DiscardUnknownMembers     bool
	Marshalers                *Marshalers
	// TODO: add other options
	Options // TODO: Use for v1 options
}

func (mo MarshalOptions) OptionsArshal(dst *jsonopts.Struct, _ internal.NotForPublicUse) {
	var bits Options
	bits = StringifyNumbers(mo.StringifyNumbers)
	dst.Flags.Set(bits.(jsonflags.Bools))
	bits = FormatNilMapAsNull(mo.FormatNilMapAsNull)
	dst.Flags.Set(bits.(jsonflags.Bools))
	dst.Marshalers = mo.Marshalers
	// TODO: Get all the other values from public struct

	// TODO: ALTERNATELY, this is what it would look like if we still need to
	// decouple from the struct.
	//stringifiers, ok := src.(interface{ getStringifyNumbers() bool })
	//if ok {
	//	bits := StringifyNumbers(stringifiers.getStringifyNumbers())
	//	dst.Flags.Set(bits.(jsonflags.Bools))
	//}
	//stringifiers, ok := src.(interface{ getFormatNilMapAsNull() bool })
	//if ok {
	//	bits := FormatNilMapAsNull(stringifiers.getFormatNilMapAsNull())
	//	dst.Flags.Set(bits.(jsonflags.Bools))
	//}
	//marshalers, ok := src.(interface{ getMarshalers() *Marshalers })
	//if ok {
	//	dst.Flags.Set(marshalers.getMarshalers())
	//}
	//
}

// TODO: ALTERNATELY, these are the get<OptProp>() methods to support the above
// commented out code.
//func (opts MarshalOptions) getFormatNilMapAsNull() bool {
//	return opts.FormatNilMapAsNull
//}
//func (opts MarshalOptions) getStringifyNumbers() bool {
//	return opts.StringifyNumbers
//}
//func (opts MarshalOptions) getMarshalers() *Marshalers {
//	return opts.Marshalers
//}

func (MarshalOptions) JSONOptions(internal.NotForPublicUse) {}

var _ OptionsArshaler = (*UnmarshalOptions)(nil)
var _ jsonopts.Options = (*UnmarshalOptions)(nil)

type UnmarshalOptions struct {
	StringifyNumbers          bool
	MatchCaseInsensitiveNames bool
	RejectUnknownMembers      bool
	Unmarshalers              *Unmarshalers
	// other options
}

func (UnmarshalOptions) JSONOptions(internal.NotForPublicUse) {}
func (mo UnmarshalOptions) OptionsArshal(dst *jsonopts.Struct, _ internal.NotForPublicUse) {
	// TODO: Get all the other values from public struct
}

var _ OptionsCoder = (*EncodeOptions)(nil)
var _ jsonopts.Options = (*EncodeOptions)(nil)

type EncodeOptions struct {
	AllowDuplicateNames bool
	AllowInvalidUTF8    bool
	EscapeForHTML       bool
	EscapeForJS         bool
	Indent              string
	IndentPrefix        string
	// other options
}

func (mo EncodeOptions) OptionsCode(dst *jsonopts.Struct, _ internal.NotForPublicUse) {
	// TODO: Get all the other values from public struct
}
func (EncodeOptions) JSONOptions(internal.NotForPublicUse) {}

var _ OptionsCoder = (*DecodeOptions)(nil)
var _ jsonopts.Options = (*DecodeOptions)(nil)

type DecodeOptions struct {
	AllowDuplicateNames bool
	AllowInvalidUTF8    bool
	// other options
}

func (DecodeOptions) OptionsCode(dst *jsonopts.Struct, _ internal.NotForPublicUse) {
	// TODO: Get all the other values from public struct
}
func (DecodeOptions) JSONOptions(internal.NotForPublicUse) {}
