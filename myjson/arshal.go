package myjson

import (
	"github.com/go-json-experiment/json"
)

// OptionsArshaler defines a method for allowing `json` to set options from
// `jsonopts` without creating an import cycle.
type OptionsArshaler interface {
	OptionsArshal(*json.OptionsStruct)
}

// OptionsCoder defines a method for allowing `json` to set options from
// `jsonopts` without creating an import cycle.
type OptionsCoder interface {
	OptionsCode(*json.OptionsStruct)
}

var _ OptionsArshaler = (*MarshalOptions)(nil)
var _ json.Options = (*MarshalOptions)(nil)

type MarshalOptions struct {
	StringifyNumbers          bool
	Deterministic             bool
	FormatNilSliceAsNull      bool
	FormatNilMapAsNull        bool
	MatchCaseInsensitiveNames bool
	DiscardUnknownMembers     bool
	Marshalers                *json.Marshalers
	// TODO: add other options
	json.Options // TODO: Use for v1 options
}

func (mo *MarshalOptions) Join(s *json.OptionsStruct, options json.Options) (unknown bool) {
	switch option := options.(type) {
	case optionsArshaler:
		option.OptionsArshal(s)
	case optionsCoder:
		option.OptionsCode(s)
	default:
		unknown = true
	}
	return unknown
}

func (mo *MarshalOptions) OptionsArshal(dst *json.OptionsStruct) {
	var bits json.Options
	bits = json.StringifyNumbers(mo.StringifyNumbers)
	dst.Flags.Set(bits.(json.BoolFlags))
	bits = json.FormatNilMapAsNull(mo.FormatNilMapAsNull)
	dst.Flags.Set(bits.(json.BoolFlags))
	dst.Marshalers = mo.Marshalers
}

func (MarshalOptions) JSONOptions() {}

var _ OptionsArshaler = (*UnmarshalOptions)(nil)
var _ json.Options = (*UnmarshalOptions)(nil)

type UnmarshalOptions struct {
	StringifyNumbers          bool
	MatchCaseInsensitiveNames bool
	RejectUnknownMembers      bool
	Unmarshalers              *json.Unmarshalers
	// other options
}

func (UnmarshalOptions) JSONOptions() {}
func (mo UnmarshalOptions) OptionsArshal(dst *json.OptionsStruct) {
	// TODO: Get all the other values from public struct
}

var _ OptionsCoder = (*EncodeOptions)(nil)
var _ json.Options = (*EncodeOptions)(nil)

type EncodeOptions struct {
	AllowDuplicateNames bool
	AllowInvalidUTF8    bool
	EscapeForHTML       bool
	EscapeForJS         bool
	Indent              string
	IndentPrefix        string
	// other options
}

func (mo EncodeOptions) OptionsCode(dst *json.OptionsStruct) {
	// TODO: Get all the other values from public struct
}
func (EncodeOptions) JSONOptions() {}

var _ OptionsCoder = (*DecodeOptions)(nil)
var _ json.Options = (*DecodeOptions)(nil)

type DecodeOptions struct {
	AllowDuplicateNames bool
	AllowInvalidUTF8    bool
	// other options
}

func (DecodeOptions) OptionsCode(dst *json.OptionsStruct) {
	// TODO: Get all the other values from public struct
}
func (DecodeOptions) JSONOptions() {}
