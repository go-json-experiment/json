// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"errors"
	"reflect"
)

// NOTE: This package supports "inlining" a Go struct field, where the contents
// of the serialized field (which must be a JSON object) are treated as if
// they are part of the parent Go struct (which represents a JSON object).
//
// Generally, inlined fields are of a Go struct type, where the fields of the
// nested struct are virtually hoisted up to the parent struct using rules
// similar to how Go embedding works (but operating on the JSON namespace).
//
// However, inlined fields may also be of a Go map type with a string key
// or a RawValue. Such inlined fields are called "fallback" fields since they
// represent any arbitrary JSON object member. Explicitly named fields take
// precedence over the inlined fallback. Only one inlined fallback is allowed.

var rawValueType = reflect.TypeOf((*RawValue)(nil)).Elem()

// marshalInlinedFallbackAll marshals all the members in an inlined fallback.
func marshalInlinedFallbackAll(mo MarshalOptions, enc *Encoder, va addressableValue, f *structField, insertQuotedName func([]byte) bool) error {
	v := addressableValue{va.Field(f.index[0])} // addressable if struct value is addressable
	if len(f.index) > 1 {
		v = v.fieldByIndex(f.index[1:], false)
		if !v.IsValid() {
			return nil // implies a nil inlined field
		}
	}
	v = v.indirect(false)
	if !v.IsValid() {
		return nil
	}

	if v.Type() == rawValueType {
		b := v.Interface().(RawValue)
		if len(b) == 0 { // TODO: Should this be nil? What if it were all whitespace?
			return nil
		}

		dec := new(Decoder) // TODO: Pool this?
		dec.reset(b, nil, DecodeOptions{AllowDuplicateNames: true, AllowInvalidUTF8: true})

		tok, err := dec.ReadToken()
		if err != nil {
			return &SemanticError{action: "marshal", GoType: rawValueType, Err: err}
		}
		if tok.Kind() != '{' {
			err := errors.New("inlined raw value must be a JSON object")
			return &SemanticError{action: "marshal", JSONKind: tok.Kind(), GoType: rawValueType, Err: err}
		}
		for dec.PeekKind() != '}' {
			val, err := dec.ReadValue()
			if err != nil {
				return &SemanticError{action: "marshal", GoType: rawValueType, Err: err}
			}
			if dec.tokens.last().needObjectValue() && insertQuotedName != nil && !insertQuotedName(val) {
				return &SyntacticError{str: "duplicate name " + string(val) + " in object"}
			}
			if err := enc.WriteValue(val); err != nil {
				return err
			}
		}
		if _, err := dec.ReadToken(); err != nil {
			return &SemanticError{action: "marshal", GoType: rawValueType, Err: err}
		}
		if err := dec.checkEOF(); err != nil {
			return &SemanticError{action: "marshal", GoType: rawValueType, Err: err}
		}
		return nil
	} else {
		if v.Len() == 0 {
			return nil
		}
		m := v
		mv := newAddressableValue(m.Type().Elem())
		for iter := m.MapRange(); iter.Next(); {
			b, err := appendString(enc.UnusedBuffer(), iter.Key().String(), !enc.options.AllowInvalidUTF8, nil)
			if err != nil {
				return err
			}
			if insertQuotedName != nil && !insertQuotedName(b) {
				return &SyntacticError{str: "duplicate name " + string(b) + " in object"}
			}
			if err := enc.WriteValue(b); err != nil {
				return err
			}

			mv.Set(iter.Value())
			marshalVal := f.fncs.marshal // TODO: Handle custom arshalers.
			if err := marshalVal(mo, enc, mv); err != nil {
				return err
			}
		}
		return nil
	}
}

// unmarshalInlinedFallbackNext unmarshals only the next member in an inlined fallback.
func unmarshalInlinedFallbackNext(uo UnmarshalOptions, dec *Decoder, va addressableValue, f *structField, rawName []byte) error {
	v := addressableValue{va.Field(f.index[0])} // addressable if struct value is addressable
	if len(f.index) > 1 {
		v = v.fieldByIndex(f.index[1:], true)
	}
	v = v.indirect(true)

	if v.Type() == rawValueType {
		b := v.Addr().Interface().(*RawValue)
		if len(*b) == 0 { // TODO: Should this be nil? What if it were all whitespace?
			*b = append(*b, '{')
		} else {
			*b = trimSuffixWhitespace(*b)
			if hasSuffixByte(*b, '}') {
				// TODO: When merging into an object for the first time,
				// should we verify that it is valid?
				*b = trimSuffixByte(*b, '}')
				*b = trimSuffixWhitespace(*b)
				if !hasSuffixByte(*b, ',') && !hasSuffixByte(*b, '{') {
					*b = append(*b, ',')
				}
			} else {
				err := errors.New("inlined raw value must be a JSON object")
				return &SemanticError{action: "unmarshal", GoType: rawValueType, Err: err}
			}
		}
		*b = append(*b, rawName...)
		*b = append(*b, ':')
		rawValue, err := dec.ReadValue()
		if err != nil {
			return err
		}
		*b = append(*b, rawValue...)
		*b = append(*b, '}')
		return nil
	} else {
		name := string(unescapeStringMayCopy(rawName)) // TODO: Intern this?

		m := v
		if m.IsNil() {
			m.Set(reflect.MakeMap(m.Type()))
		}
		mk := reflect.ValueOf(name)
		mv := newAddressableValue(v.Type().Elem()) // TODO: Cache across calls?
		if v2 := m.MapIndex(mk); v2.IsValid() {
			mv.Set(v2)
		}

		unmarshal := f.fncs.unmarshal // TODO: Handle custom arshalers.
		err := unmarshal(uo, dec, mv)
		m.SetMapIndex(mk, mv.Value)
		if err != nil {
			return err
		}
		return nil
	}
}
