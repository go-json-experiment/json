// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json_test

import (
	"fmt"
	"log"
	"reflect"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

// OrderedObject is an ordered sequence of name/value members in a JSON object.
//
// RFC 8259 defines an object as an "unordered collection".
// JSON implementations need not make "ordering of object members visible"
// to applications nor will they agree on the semantic meaning of an object if
// "the names within an object are not unique". For maximum compatibility,
// applications should avoid relying on ordering or duplicity of object names.
type OrderedObject[V any] []ObjectMember[V]

// ObjectMember is a JSON object member.
type ObjectMember[V any] struct {
	Name  string
	Value V
}

// MarshalJSONTo encodes obj as a JSON object into enc.
func (obj *OrderedObject[V]) MarshalJSONTo(enc *jsontext.Encoder, opts json.Options) error {
	if err := enc.WriteToken(jsontext.ObjectStart); err != nil {
		return err
	}
	for i := range *obj {
		member := &(*obj)[i]
		if err := json.MarshalEncode(enc, &member.Name, opts); err != nil {
			return err
		}
		if err := json.MarshalEncode(enc, &member.Value, opts); err != nil {
			return err
		}
	}
	if err := enc.WriteToken(jsontext.ObjectEnd); err != nil {
		return err
	}
	return nil
}

// UnmarshalJSONFrom decodes a JSON object from dec into obj.
func (obj *OrderedObject[V]) UnmarshalJSONFrom(dec *jsontext.Decoder, opts json.Options) error {
	if k := dec.PeekKind(); k != '{' {
		return fmt.Errorf("expected object start, but encountered %v", k)
	}
	if _, err := dec.ReadToken(); err != nil {
		return err
	}
	for dec.PeekKind() != '}' {
		*obj = append(*obj, ObjectMember[V]{})
		member := &(*obj)[len(*obj)-1]
		if err := json.UnmarshalDecode(dec, &member.Name, opts); err != nil {
			return err
		}
		if err := json.UnmarshalDecode(dec, &member.Value, opts); err != nil {
			return err
		}
	}
	if _, err := dec.ReadToken(); err != nil {
		return err
	}
	return nil
}

// The exact order of JSON object can be preserved through the use of a
// specialized type that implements [MarshalerTo] and [UnmarshalerFrom].
func Example_orderedObject() {
	// Round-trip marshal and unmarshal an ordered object.
	// We expect the order and duplicity of JSON object members to be preserved.
	// Specify jsontext.AllowDuplicateNames since this object contains "fizz" twice.
	want := OrderedObject[string]{
		{"fizz", "buzz"},
		{"hello", "world"},
		{"fizz", "wuzz"},
	}
	b, err := json.Marshal(&want, jsontext.AllowDuplicateNames(true))
	if err != nil {
		log.Fatal(err)
	}
	var got OrderedObject[string]
	err = json.Unmarshal(b, &got, jsontext.AllowDuplicateNames(true))
	if err != nil {
		log.Fatal(err)
	}

	// Sanity check.
	if !reflect.DeepEqual(got, want) {
		log.Fatalf("roundtrip mismatch: got %v, want %v", got, want)
	}

	// Print the serialized JSON object.
	(*jsontext.Value)(&b).Indent("", "\t") // indent for readability
	fmt.Println(string(b))

	// Output:
	// {
	// 	"fizz": "buzz",
	// 	"hello": "world",
	// 	"fizz": "wuzz"
	// }
}
