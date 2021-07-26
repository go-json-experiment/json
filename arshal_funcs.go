// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"encoding"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

var (
	encoderPointerType   = reflect.TypeOf((*Encoder)(nil))
	decoderPointerType   = reflect.TypeOf((*Decoder)(nil))
	marshalOptionsType   = reflect.TypeOf((*MarshalOptions)(nil)).Elem()
	unmarshalOptionsType = reflect.TypeOf((*UnmarshalOptions)(nil)).Elem()
	marshalersType       = reflect.TypeOf((*Marshalers)(nil)).Elem()
	unmarshalersType     = reflect.TypeOf((*Unmarshalers)(nil)).Elem()
	bytesType            = reflect.TypeOf((*[]byte)(nil)).Elem()
	errorType            = reflect.TypeOf((*error)(nil)).Elem()

	// Most natural Go type that correspond with each JSON type.
	boolType           = reflect.TypeOf((*bool)(nil)).Elem()                   // JSON bool
	stringType         = reflect.TypeOf((*string)(nil)).Elem()                 // JSON string
	float64Type        = reflect.TypeOf((*float64)(nil)).Elem()                // JSON number
	mapStringIfaceType = reflect.TypeOf((*map[string]interface{})(nil)).Elem() // JSON object
	sliceIfaceType     = reflect.TypeOf((*[]interface{})(nil)).Elem()          // JSON array

	// Interfaces for custom serialization.
	jsonMarshalerV1Type   = reflect.TypeOf((*MarshalerV1)(nil)).Elem()
	jsonMarshalerV2Type   = reflect.TypeOf((*MarshalerV2)(nil)).Elem()
	jsonUnmarshalerV1Type = reflect.TypeOf((*UnmarshalerV1)(nil)).Elem()
	jsonUnmarshalerV2Type = reflect.TypeOf((*UnmarshalerV2)(nil)).Elem()
	textMarshalerType     = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	textUnmarshalerType   = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
)

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

// addrWhen returns va.Addr if addr is specified, otherwise it returns itself.
func (va addressableValue) addrWhen(addr bool) reflect.Value {
	if addr {
		return va.Addr()
	}
	return va.Value
}

// implementsWhich is like t.Implements(ifaceType) for a list of interfaces,
// but checks whether either t or reflect.PtrTo(t) implements the interface.
// It returns the first interface type that matches and whether a value of t
// needs to be addressed first before it implements the interface.
func implementsWhich(t reflect.Type, ifaceTypes ...reflect.Type) (which reflect.Type, needAddr bool) {
	for _, ifaceType := range ifaceTypes {
		switch {
		case t.Implements(ifaceType):
			return ifaceType, false
		case reflect.PtrTo(t).Implements(ifaceType):
			return ifaceType, true
		}
	}
	return nil, false
}

// All marshal and unmarshal behavior is implemented using these signatures.
type (
	marshaler   func(MarshalOptions, *Encoder, addressableValue) error
	unmarshaler func(UnmarshalOptions, *Decoder, addressableValue) error
)

type arshaler struct {
	marshal   marshaler
	unmarshal unmarshaler
}

var lookupArshalerCache sync.Map // map[reflect.Type]*arshaler

func lookupArshaler(t reflect.Type) *arshaler {
	if v, ok := lookupArshalerCache.Load(t); ok {
		return v.(*arshaler)
	}

	fncs := makeDefaultArshaler(t)
	fncs = makeMethodArshaler(fncs, t)

	// Use the last stored so that duplicate arshalers can be garbage collected.
	v, _ := lookupArshalerCache.LoadOrStore(t, fncs)
	return v.(*arshaler)
}

func makeDefaultArshaler(t reflect.Type) *arshaler {
	switch t.Kind() {
	case reflect.Bool:
		return makeBoolArshaler(t)
	case reflect.String:
		return makeStringArshaler(t)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return makeIntArshaler(t)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return makeUintArshaler(t)
	case reflect.Float32, reflect.Float64:
		return makeFloatArshaler(t)
	case reflect.Map:
		return makeMapArshaler(t)
	case reflect.Struct:
		return makeStructArshaler(t)
	case reflect.Slice:
		if t.AssignableTo(bytesType) {
			return makeBytesArshaler(t)
		}
		return makeSliceArshaler(t)
	case reflect.Array:
		if reflect.SliceOf(t.Elem()).AssignableTo(bytesType) {
			return makeBytesArshaler(t)
		}
		return makeArrayArshaler(t)
	case reflect.Ptr:
		return makePtrArshaler(t)
	case reflect.Interface:
		return makeInterfaceArshaler(t)
	default:
		return makeInvalidArshaler(t)
	}
}

func makeBoolArshaler(t reflect.Type) *arshaler {
	var fncs arshaler
	fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
		return enc.WriteToken(Bool(va.Bool()))
	}
	fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}
		k := tok.Kind()
		switch k {
		case 'n':
			va.SetBool(false)
			return nil
		case 't', 'f':
			va.SetBool(tok.Bool())
			return nil
		}
		return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t}
	}
	return &fncs
}

func makeStringArshaler(t reflect.Type) *arshaler {
	var fncs arshaler
	fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
		return enc.WriteToken(String(va.String()))
	}
	fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}
		k := tok.Kind()
		switch k {
		case 'n':
			va.SetString("")
			return nil
		case '"':
			va.SetString(tok.String())
			return nil
		}
		return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t}
	}
	return &fncs
}

func makeBytesArshaler(t reflect.Type) *arshaler {
	// NOTE: This handles both []byte and [N]byte.
	var fncs arshaler
	fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
		val := enc.UnusedBuffer()
		var b []byte
		if t.Kind() == reflect.Array {
			b = va.Slice(0, t.Len()).Bytes()
		} else {
			b = va.Bytes()
		}
		n := len(`"`) + base64.StdEncoding.EncodedLen(len(b)) + len(`"`)
		if cap(val) < n {
			val = make([]byte, n)
		} else {
			val = val[:n]
		}
		val[0] = '"'
		base64.StdEncoding.Encode(val[len(`"`):len(val)-len(`"`)], b)
		val[len(val)-1] = '"'
		return enc.WriteValue(val)
	}
	fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
		val, err := dec.ReadValue()
		if err != nil {
			return err
		}
		k := val.Kind()
		switch k {
		case 'n':
			va.Set(reflect.Zero(t))
			return nil
		case '"':
			val = unescapeSimpleString(val)

			// NOTE: StdEncoding.DecodedLen reports the maximum decoded length
			// for padded encoding schemes since it cannot determine
			// how many characters at the end are for padding.
			// To compute the exact count, use RawStdEncoding.DecodedLen instead
			// on the input size with padding already discounted.
			rawLen := len(val)
			for rawLen > 0 && val[rawLen-1] == '=' {
				rawLen--
			}
			n := base64.RawStdEncoding.DecodedLen(rawLen)

			var b []byte
			if t.Kind() == reflect.Array {
				b = va.Slice(0, t.Len()).Bytes()
				if n != len(b) {
					err := fmt.Errorf("decoded base64 length of %d mismatches array length of %d", n, t.Len())
					return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t, Err: err}
				}
			} else {
				b = va.Bytes()
				if b == nil || cap(b) < n {
					b = make([]byte, n)
				} else {
					b = b[:n]
				}
			}
			if _, err := base64.StdEncoding.Decode(b, val); err != nil {
				return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t, Err: err}
			}
			if t.Kind() == reflect.Slice {
				va.SetBytes(b)
			}
			return nil
		}
		return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t}
	}
	return &fncs
}

func makeIntArshaler(t reflect.Type) *arshaler {
	var fncs arshaler
	fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
		val := enc.UnusedBuffer()
		if mo.StringifyNumbers {
			val = append(val, '"')
		}
		val = strconv.AppendInt(val, va.Int(), 10)
		if mo.StringifyNumbers {
			val = append(val, '"')
		}
		return enc.WriteValue(val)
	}
	fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
		val, err := dec.ReadValue()
		if err != nil {
			return err
		}
		k := val.Kind()
		switch k {
		case 'n':
			va.SetInt(0)
			return nil
		case '"':
			if !uo.StringifyNumbers {
				break
			}
			val = unescapeSimpleString(val)
			fallthrough
		case '0':
			var negOffset int
			neg := val[0] == '-'
			if neg {
				negOffset = 1
			}
			n, ok := parseDecUint(val[negOffset:])
			maxInt := uint64(1) << (t.Bits() - 1)
			overflow := (neg && n > maxInt) || (!neg && n > maxInt-1)
			if !ok {
				if n != math.MaxUint64 {
					err := fmt.Errorf("cannot parse %q as signed integer: %w", val, strconv.ErrSyntax)
					return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t, Err: err}
				}
				overflow = true
			}
			if overflow {
				err := fmt.Errorf("cannot parse %q as signed integer: %w", val, strconv.ErrRange)
				return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t, Err: err}
			}
			if neg {
				va.SetInt(int64(-n))
			} else {
				va.SetInt(int64(+n))
			}
			return nil
		}
		return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t}
	}
	return &fncs
}

func makeUintArshaler(t reflect.Type) *arshaler {
	var fncs arshaler
	fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
		val := enc.UnusedBuffer()
		if mo.StringifyNumbers {
			val = append(val, '"')
		}
		val = strconv.AppendUint(val, va.Uint(), 10)
		if mo.StringifyNumbers {
			val = append(val, '"')
		}
		return enc.WriteValue(val)
	}
	fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
		val, err := dec.ReadValue()
		if err != nil {
			return err
		}
		k := val.Kind()
		switch k {
		case 'n':
			va.SetUint(0)
			return nil
		case '"':
			if !uo.StringifyNumbers {
				break
			}
			val = unescapeSimpleString(val)
			fallthrough
		case '0':
			n, ok := parseDecUint(val)
			maxUint := uint64(1) << t.Bits()
			overflow := n > maxUint-1
			if !ok {
				if n != math.MaxUint64 {
					err := fmt.Errorf("cannot parse %q as unsigned integer: %w", val, strconv.ErrSyntax)
					return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t, Err: err}
				}
				overflow = true
			}
			if overflow {
				err := fmt.Errorf("cannot parse %q as unsigned integer: %w", val, strconv.ErrRange)
				return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t, Err: err}
			}
			va.SetUint(uint64(n))
			return nil
		}
		return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t}
	}
	return &fncs
}

func makeFloatArshaler(t reflect.Type) *arshaler {
	var fncs arshaler
	fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
		fv := va.Float()
		if math.IsNaN(fv) || math.IsInf(fv, 0) {
			err := fmt.Errorf("invalid value: %v", fv)
			return &SemanticError{action: "marshal", GoType: t, Err: err}
		}
		val := enc.UnusedBuffer()
		if mo.StringifyNumbers {
			val = append(val, '"')
		}
		val = appendNumber(val, fv, t.Bits())
		if mo.StringifyNumbers {
			val = append(val, '"')
		}
		return enc.WriteValue(val)
	}
	fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
		val, err := dec.ReadValue()
		if err != nil {
			return err
		}
		k := val.Kind()
		switch k {
		case 'n':
			va.SetFloat(0)
			return nil
		case '"':
			if !uo.StringifyNumbers {
				break
			}
			val = unescapeSimpleString(val)
			if n, err := consumeNumber(val); n != len(val) || err != nil {
				err := fmt.Errorf("cannot parse %q as JSON number: %w", val, strconv.ErrSyntax)
				return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t, Err: err}
			}
			fallthrough
		case '0':
			// NOTE: Floating-point parsing is by nature a lossy operation.
			// We never report an overflow condition since we can always
			// round the input to the closest representable finite value.
			// For extremely large numbers, the closest value is ±MaxFloat.
			fv, _ := parseFloat(val, t.Bits())
			va.SetFloat(fv)
			return nil
		}
		return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t}
	}
	return &fncs
}

func makeMapArshaler(t reflect.Type) *arshaler {
	// NOTE: Values retrieved from a map are not addressable,
	// so we shallow copy the values to make them addressable and
	// store them back into the map afterwards.
	var fncs arshaler
	var (
		once    sync.Once
		keyFncs *arshaler
		valFncs *arshaler
	)
	init := func() {
		keyFncs = lookupArshaler(t.Key())
		valFncs = lookupArshaler(t.Elem())
	}
	fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
		// TODO: Perform depth check and cycle detection.
		if err := enc.WriteToken(ObjectStart); err != nil {
			return err
		}
		if va.Len() > 0 {
			// Handle maps with numeric key types by stringifying them.
			mko := mo
			mko.StringifyNumbers = true

			once.Do(init)
			// TODO: Handle custom arshalers.
			marshalKey := keyFncs.marshal
			marshalVal := valFncs.marshal
			k := newAddressableValue(t.Key())
			v := newAddressableValue(t.Elem())
			// NOTE: Map entries are serialized in a non-deterministic order.
			// Users that need stable output should call RawValue.Canonicalize.
			for iter := va.MapRange(); iter.Next(); {
				k.Set(iter.Key())
				if err := marshalKey(mko, enc, k); err != nil {
					// TODO: If err is errMissingName, then wrap it with as a
					// SemanticError since this key type cannot be serialized
					// as a JSON string.
					return err
				}
				v.Set(iter.Value())
				if err := marshalVal(mo, enc, v); err != nil {
					return err
				}
			}
		}
		if err := enc.WriteToken(ObjectEnd); err != nil {
			return err
		}
		return nil
	}
	fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}
		k := tok.Kind()
		switch k {
		case 'n':
			va.Set(reflect.Zero(t))
			return nil
		case '{':
			if va.IsNil() {
				va.Set(reflect.MakeMap(t))
			}

			// Handle maps with numeric key types by stringifying them.
			uko := uo
			uko.StringifyNumbers = true

			once.Do(init)
			// TODO: Handle custom arshalers.
			unmarshalKey := keyFncs.unmarshal
			unmarshalVal := valFncs.unmarshal
			k := newAddressableValue(t.Key())
			v := newAddressableValue(t.Elem())
			for dec.PeekKind() != '}' {
				k.Set(reflect.Zero(t.Key()))
				if err := unmarshalKey(uko, dec, k); err != nil {
					return err
				}
				if k.Kind() == reflect.Interface && !k.IsNil() && !k.Elem().Type().Comparable() {
					err := fmt.Errorf("invalid incomparable key type %v", k.Elem().Type())
					return &SemanticError{action: "unmarshal", GoType: t, Err: err}
				}

				if v2 := va.MapIndex(k.Value); v2.IsValid() {
					v.Set(v2)
				} else {
					v.Set(reflect.Zero(v.Type()))
				}
				err := unmarshalVal(uo, dec, v)
				va.SetMapIndex(k.Value, v.Value)
				if err != nil {
					return err
				}
			}
			if _, err := dec.ReadToken(); err != nil {
				return err
			}
			return nil
		}
		return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t}
	}
	return &fncs
}

func makeStructArshaler(t reflect.Type) *arshaler {
	// TODO: Support `inline`, `unknown`, and `format`.
	// TODO: Support MarshalOptions.DiscardUnknownMembers and UnmarshalOptions.RejectUnknownMembers.
	var fncs arshaler
	type field struct {
		index int // index into reflect.StructField.Field
		fncs  *arshaler
		fieldOptions
	}
	var (
		once         sync.Once
		fields       []field
		fieldsByName map[string]int // index into fields slice
		hasNocase    bool           // does any field have `nocase` specified?
		errInit      *SemanticError
	)
	init := func() {
		var hasAnyJSONTag bool
		fieldsByName = make(map[string]int, t.NumField())
		for i := 0; i < t.NumField(); i++ {
			sf := t.Field(i)
			_, hasTag := sf.Tag.Lookup("json")
			hasAnyJSONTag = hasAnyJSONTag || hasTag
			options, err := parseFieldOptions(sf)
			if err != nil {
				if err == errIgnoredField {
					continue
				}
				errInit = &SemanticError{GoType: t, Err: err}
				return
			}
			if j, ok := fieldsByName[options.name]; ok {
				err := fmt.Errorf("Go struct fields %q and %q conflict over JSON object name %q", t.Field(j).Name, t.Field(i).Name, options.name)
				errInit = &SemanticError{GoType: t, Err: err}
				return
			}
			fieldsByName[options.name] = len(fields)
			fields = append(fields, field{
				index:        i,
				fncs:         lookupArshaler(sf.Type),
				fieldOptions: options,
			})
			hasNocase = hasNocase || options.nocase
		}

		// NOTE: New users to the json package are occasionally surprised that
		// unexported fields are ignored. This occurs by necessity due to our
		// inability to directly introspect such fields with Go reflection
		// without the use of unsafe.
		//
		// To reduce friction here, refuse to serialize any Go struct that
		// has no JSON serializable fields, has at least one Go struct field,
		// and does not have any `json` tags present. For example,
		// errors returned by errors.New would fail to serialize.
		if len(fields) == 0 && t.NumField() > 0 && !hasAnyJSONTag {
			err := errors.New("Go struct kind has no exported fields")
			errInit = &SemanticError{GoType: t, Err: err}
			return
		}
	}
	fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
		// TODO: Perform depth check and cycle detection.
		if err := enc.WriteToken(ObjectStart); err != nil {
			return err
		}
		once.Do(init)
		if errInit != nil {
			err := *errInit // shallow copy SemanticError
			err.action = "marshal"
			return &err
		}
		for _, f := range fields {
			v := addressableValue{va.Field(f.index)} // addressable if struct value is addressable
			if f.omitzero && v.IsZero() {
				continue
			}
			// TODO: Support `omitempty`.
			if err := enc.WriteToken(String(f.name)); err != nil {
				return err
			}
			marshal := f.fncs.marshal // TODO: Handle custom arshalers.
			mo2 := mo
			mo2.StringifyNumbers = mo2.StringifyNumbers || f.string
			if err := marshal(mo2, enc, v); err != nil {
				return err
			}
		}
		if err := enc.WriteToken(ObjectEnd); err != nil {
			return err
		}
		return nil
	}
	fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}
		k := tok.Kind()
		switch k {
		case 'n':
			va.Set(reflect.Zero(t))
			return nil
		case '{':
			once.Do(init)
			if errInit != nil {
				err := *errInit // shallow copy SemanticError
				err.action = "unmarshal"
				return &err
			}
			for dec.PeekKind() != '}' {
				// Process the object member name.
				val, err := dec.ReadValue()
				if err != nil {
					return err
				}
				name := unescapeSimpleString(val)
				i, ok := fieldsByName[string(name)]
				if !ok && hasNocase {
					for i2, f := range fields {
						if f.nocase && strings.EqualFold(f.name, string(name)) {
							i, ok = i2, true
							break
						}
					}
				}
				if !ok {
					// Consume unknown object member.
					if _, err := dec.ReadValue(); err != nil {
						return err
					}
					continue
				}
				f := fields[i]

				// Process the object member value.
				unmarshal := f.fncs.unmarshal // TODO: Handle custom arshalers.
				uo2 := uo
				uo2.StringifyNumbers = uo2.StringifyNumbers || f.string
				v := addressableValue{va.Field(f.index)} // addressable if struct value is addressable
				if err := unmarshal(uo2, dec, v); err != nil {
					return err
				}
			}
			if _, err := dec.ReadToken(); err != nil {
				return err
			}
			return nil
		}
		return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t}
	}
	return &fncs
}

func makeSliceArshaler(t reflect.Type) *arshaler {
	var fncs arshaler
	var (
		once    sync.Once
		valFncs *arshaler
	)
	init := func() {
		valFncs = lookupArshaler(t.Elem())
	}
	fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
		// TODO: Perform depth check and cycle detection.
		if err := enc.WriteToken(ArrayStart); err != nil {
			return err
		}
		once.Do(init)
		marshal := valFncs.marshal // TODO: Handle custom arshalers.
		for i := 0; i < va.Len(); i++ {
			v := addressableValue{va.Index(i)} // indexed slice element is always addressable
			if err := marshal(mo, enc, v); err != nil {
				return err
			}
		}
		if err := enc.WriteToken(ArrayEnd); err != nil {
			return err
		}
		return nil
	}
	fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}
		k := tok.Kind()
		switch k {
		case 'n':
			va.Set(reflect.Zero(t))
			return nil
		case '[':
			once.Do(init)
			unmarshal := valFncs.unmarshal // TODO: Handle custom arshalers.
			va.Set(va.Slice(0, 0))
			var i int
			for dec.PeekKind() != ']' {
				va.Set(reflect.Append(va.Value, reflect.Zero(t.Elem())))
				v := addressableValue{va.Index(i)} // indexed slice element is always addressable
				if err := unmarshal(uo, dec, v); err != nil {
					return err
				}
				i++
			}
			if va.IsNil() {
				va.Set(reflect.MakeSlice(va.Type(), 0, 0))
			}
			if _, err := dec.ReadToken(); err != nil {
				return err
			}
			return nil
		}
		return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t}
	}
	return &fncs
}

func makeArrayArshaler(t reflect.Type) *arshaler {
	var fncs arshaler
	var (
		once    sync.Once
		valFncs *arshaler
	)
	init := func() {
		valFncs = lookupArshaler(t.Elem())
	}
	fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
		// TODO: Perform depth check and cycle detection.
		if err := enc.WriteToken(ArrayStart); err != nil {
			return err
		}
		once.Do(init)
		marshal := valFncs.marshal // TODO: Handle custom arshalers.
		for i := 0; i < t.Len(); i++ {
			v := addressableValue{va.Index(i)} // indexed array element is addressable if array is addressable
			if err := marshal(mo, enc, v); err != nil {
				return err
			}
		}
		if err := enc.WriteToken(ArrayEnd); err != nil {
			return err
		}
		return nil
	}
	fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}
		k := tok.Kind()
		switch k {
		case 'n':
			va.Set(reflect.Zero(t))
			return nil
		case '[':
			once.Do(init)
			unmarshal := valFncs.unmarshal // TODO: Handle custom arshalers.
			var i int
			for dec.PeekKind() != ']' {
				if i >= t.Len() {
					err := errors.New("too many array elements")
					return &SemanticError{action: "unmarshal", GoType: t, Err: err}
				}
				v := addressableValue{va.Index(i)} // indexed array element is addressable if array is addressable
				v.Set(reflect.Zero(v.Type()))
				if err := unmarshal(uo, dec, v); err != nil {
					return err
				}
				i++
			}
			if _, err := dec.ReadToken(); err != nil {
				return err
			}
			if i < t.Len() {
				err := errors.New("too few array elements")
				return &SemanticError{action: "unmarshal", GoType: t, Err: err}
			}
			return nil
		}
		return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t}
	}
	return &fncs
}

func makePtrArshaler(t reflect.Type) *arshaler {
	var fncs arshaler
	var (
		once    sync.Once
		valFncs *arshaler
	)
	init := func() {
		valFncs = lookupArshaler(t.Elem())
	}
	fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
		if va.IsNil() {
			return enc.WriteToken(Null)
		}
		once.Do(init)
		marshal := valFncs.marshal       // TODO: Handle custom arshalers. Should this occur before the nil check?
		v := addressableValue{va.Elem()} // dereferenced pointer is always addressable
		return marshal(mo, enc, v)
	}
	fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
		if dec.PeekKind() == 'n' {
			if _, err := dec.ReadToken(); err != nil {
				return err
			}
			va.Set(reflect.Zero(t))
			return nil
		}
		once.Do(init)
		unmarshal := valFncs.unmarshal // TODO: Handle custom arshalers. Should this occur before the nil check?
		if va.IsNil() {
			va.Set(reflect.New(t.Elem()))
		}
		v := addressableValue{va.Elem()} // dereferenced pointer is always addressable
		return unmarshal(uo, dec, v)
	}
	return &fncs
}

func makeInterfaceArshaler(t reflect.Type) *arshaler {
	// NOTE: Values retrieved from an interface are not addressable,
	// so we shallow copy the values to make them addressable and
	// store them back into the interface afterwards.
	var fncs arshaler
	fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
		if va.IsNil() {
			return enc.WriteToken(Null)
		}
		v := newAddressableValue(va.Elem().Type())
		v.Set(va.Elem())
		marshal := lookupArshaler(v.Type()).marshal // TODO: Handle custom arshalers. Should this occur before the nil check?
		return marshal(mo, enc, v)
	}
	fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
		if dec.PeekKind() == 'n' {
			if _, err := dec.ReadToken(); err != nil {
				return err
			}
			va.Set(reflect.Zero(t))
			return nil
		}
		var v addressableValue
		if va.IsNil() {
			k := dec.PeekKind()
			if t.NumMethod() > 0 {
				// TODO: If types sets are allowed in ordinary interface types,
				// then the concrete type to use can be known in the case where
				// the type set contains exactly one Go type.
				// See https://golang.org/issue/45346.
				err := errors.New("cannot derive concrete type for non-empty interface")
				return &SemanticError{action: "unmarshal", JSONKind: k, GoType: t, Err: err}
			}
			switch k {
			case 'f', 't':
				v = newAddressableValue(boolType)
			case '"':
				v = newAddressableValue(stringType)
			case '0':
				v = newAddressableValue(float64Type)
			case '{':
				v = newAddressableValue(mapStringIfaceType)
			case '[':
				v = newAddressableValue(sliceIfaceType)
			default:
				// TODO: This could also be due to an I/O error.
				return &SyntaxError{Offset: dec.InputOffset(), str: "invalid JSON token"}
			}
		} else {
			// Shallow copy the existing value to keep it addressable.
			// Any mutations at the top-level of the value will be observable
			// since we always store this value back into the interface value.
			v = newAddressableValue(va.Elem().Type())
			v.Set(va.Elem())
		}
		unmarshal := lookupArshaler(v.Type()).unmarshal // TODO: Handle custom arshalers. Should this occur before the nil check?
		err := unmarshal(uo, dec, v)
		va.Set(v.Value)
		return err
	}
	return &fncs
}

func makeInvalidArshaler(t reflect.Type) *arshaler {
	var fncs arshaler
	fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
		return &SemanticError{action: "marshal", GoType: t}
	}
	fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
		return &SemanticError{action: "unmarshal", GoType: t}
	}
	return &fncs
}

func makeMethodArshaler(fncs *arshaler, t reflect.Type) *arshaler {
	// Avoid injecting method arshaler on the pointer or interface version
	// to avoid ever calling the method on a nil pointer or interface receiver.
	// Let it be injected on the value receiver (which is always addressable).
	if t.Kind() == reflect.Ptr || t.Kind() == reflect.Interface {
		return fncs
	}

	// Handle custom marshaler.
	switch which, needAddr := implementsWhich(t, jsonMarshalerV2Type, jsonMarshalerV1Type, textMarshalerType); which {
	case jsonMarshalerV2Type:
		fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
			prevDepth, prevLength := enc.tokens.depthLength()
			err := va.addrWhen(needAddr).Interface().(MarshalerV2).MarshalNextJSON(enc, mo)
			currDepth, currLength := enc.tokens.depthLength()
			if (prevDepth != currDepth || prevLength+1 != currLength) && err == nil {
				err = errors.New("must write exactly one JSON value")
			}
			if err != nil {
				// TODO: Avoid wrapping semantic or I/O errors.
				return &SemanticError{action: "marshal", GoType: t, Err: err}
			}
			return nil
		}
	case jsonMarshalerV1Type:
		fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
			marshaler := va.addrWhen(needAddr).Interface().(MarshalerV1)
			val, err := marshaler.MarshalJSON()
			if err != nil {
				// TODO: Avoid wrapping semantic errors.
				return &SemanticError{action: "marshal", GoType: t, Err: err}
			}
			if err := enc.WriteValue(val); err != nil {
				// TODO: Avoid wrapping semantic or I/O errors.
				return &SemanticError{action: "marshal", JSONKind: RawValue(val).Kind(), GoType: t, Err: err}
			}
			return nil
		}
	case textMarshalerType:
		fncs.marshal = func(mo MarshalOptions, enc *Encoder, va addressableValue) error {
			marshaler := va.addrWhen(needAddr).Interface().(encoding.TextMarshaler)
			s, err := marshaler.MarshalText()
			if err != nil {
				// TODO: Avoid wrapping semantic errors.
				return &SemanticError{action: "marshal", JSONKind: '"', GoType: t, Err: err}
			}
			val := enc.UnusedBuffer()
			val, err = appendString(val, string(s), true, nil)
			if err != nil {
				return &SemanticError{action: "marshal", JSONKind: '"', GoType: t, Err: err}
			}
			if err := enc.WriteValue(val); err != nil {
				// TODO: Avoid wrapping I/O errors.
				return &SemanticError{action: "marshal", JSONKind: '"', GoType: t, Err: err}
			}
			return nil
		}
	}

	// Handle custom unmarshaler.
	switch which, needAddr := implementsWhich(t, jsonUnmarshalerV2Type, jsonUnmarshalerV1Type, textUnmarshalerType); which {
	case jsonUnmarshalerV2Type:
		fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
			prevDepth, prevLength := dec.tokens.depthLength()
			err := va.addrWhen(needAddr).Interface().(UnmarshalerV2).UnmarshalNextJSON(dec, uo)
			currDepth, currLength := dec.tokens.depthLength()
			if (prevDepth != currDepth || prevLength+1 != currLength) && err == nil {
				err = errors.New("must read exactly one JSON value")
			}
			if err != nil {
				// TODO: Avoid wrapping semantic, syntactic, or I/O errors.
				return &SemanticError{action: "unmarshal", GoType: t, Err: err}
			}
			return nil
		}
	case jsonUnmarshalerV1Type:
		fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
			val, err := dec.ReadValue()
			if err != nil {
				return err // must be a syntactic or I/O error
			}
			unmarshaler := va.addrWhen(needAddr).Interface().(UnmarshalerV1)
			if err := unmarshaler.UnmarshalJSON(val); err != nil {
				// TODO: Avoid wrapping semantic, syntactic, or I/O errors.
				return &SemanticError{action: "unmarshal", JSONKind: val.Kind(), GoType: t, Err: err}
			}
			return nil
		}
	case textUnmarshalerType:
		fncs.unmarshal = func(uo UnmarshalOptions, dec *Decoder, va addressableValue) error {
			val, err := dec.ReadValue()
			if err != nil {
				return err // must be a syntactic or I/O error
			}
			if val.Kind() != '"' {
				err = errors.New("JSON value must be string type")
				return &SemanticError{action: "unmarshal", JSONKind: val.Kind(), GoType: t, Err: err}
			}
			s := unescapeSimpleString(val)
			unmarshaler := va.addrWhen(needAddr).Interface().(encoding.TextUnmarshaler)
			if err := unmarshaler.UnmarshalText(s); err != nil {
				// TODO: Avoid wrapping semantic, syntactic, or I/O errors.
				return &SemanticError{action: "unmarshal", JSONKind: val.Kind(), GoType: t, Err: err}
			}
			return nil
		}
	}

	return fncs
}
