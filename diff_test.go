// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json_test

import (
	"path"
	"reflect"
	"testing"

	jsonv1 "encoding/json"

	jsonv2 "github.com/go-json-experiment/json"
)

// NOTE: This file serves as a list of semantic differences between v1 and v2.
// Each test explains how v1 behaves, how v2 behaves, and
// a rationale for why the behavior was changed.

var jsonPackages = []struct {
	Version   string
	Marshal   func(any) ([]byte, error)
	Unmarshal func([]byte, any) error
}{
	{"v1", jsonv1.Marshal, jsonv1.Unmarshal},
	{"v2", jsonv2.Marshal, jsonv2.Unmarshal},
}

// In v1, struct fields are identified using a case-insensitive match.
// In v2, struct fields are identified using a case-sensitive match
// unless the `nocase` tag is specified.
//
// Case-insensitive matching is a surprising default and
// incurs significant performance cost when unmarshaling unknown fields.
// In v2, we switch the default and provide the ability to opt into v1 behavior.
//
// Related issue: https://golang.org/issue/14750
func TestCaseSensitivity(t *testing.T) {
	type Fields struct {
		FieldA bool
		FieldB bool `json:"BAR"`
		FieldC bool `json:"baz,nocase"` // `nocase` is used by v2 to explicitly enable case-insensitive matching
	}

	for _, json := range jsonPackages {
		t.Run(path.Join("Unmarshal", json.Version), func(t *testing.T) {
			// This is a mapping from Go field names to JSON member names to
			// whether the JSON member name would match the Go field name.
			type goName = string
			type jsonName = string
			isV1 := json.Version == "v1"
			allMatches := map[goName]map[jsonName]bool{
				"FieldA": {
					"FieldA": true, // exact match
					"fielda": isV1, // v1 is case-insensitive by default
					"fieldA": isV1, // v1 is case-insensitive by default
					"FIELDA": isV1, // v1 is case-insensitive by default
					"FieldB": false,
					"FieldC": false,
				},
				"FieldB": {
					"bar":    isV1, // v1 is case-insensitive even if an explicit JSON name is provided
					"Bar":    isV1, // v1 is case-insensitive even if an explicit JSON name is provided
					"BAR":    true, // exact match for explicitly specified JSON name
					"baz":    false,
					"FieldA": false,
					"FieldB": false, // explicit JSON name means that the Go field name is not used for matching
					"FieldC": false,
				},
				"FieldC": {
					"baz":    true, // exact match for explicitly specified JSON name
					"Baz":    true, // v2 is case-insensitive due to `nocase` tag
					"BAZ":    true, // v2 is case-insensitive due to `nocase` tag
					"bar":    false,
					"FieldA": false,
					"FieldC": false, // explicit JSON name means that the Go field name is not used for matching
					"FieldB": false,
				},
			}

			for goFieldName, matches := range allMatches {
				for jsonMemberName, wantMatch := range matches {
					in := `{"` + jsonMemberName + `":true}`
					var s Fields
					if err := json.Unmarshal([]byte(in), &s); err != nil {
						t.Fatalf("json.Unmarshal error: %v", err)
					}
					gotMatch := reflect.ValueOf(s).FieldByName(goFieldName).Bool()
					if gotMatch != wantMatch {
						t.Fatalf("%T.%s = %v, want %v", s, goFieldName, gotMatch, wantMatch)
					}
				}
			}
		})
	}
}

// In v1, nil slices and maps are marshaled as a JSON null.
// In v2, nil slices and maps are marshaled as an empty JSON object or array.
//
// JSON is a language-agnostic data interchange format.
// The fact that maps and slices are nil-able in Go is a semantic detail of the
// Go language. We should avoid leaking such details to the JSON representation.
// When JSON implementations leak language-specific details, it complicates
// transition to/from languages with different type systems.
//
// Furthermore, consider two related Go types: string and []byte.
// It's an asymmetric oddity of v1 that zero values of string and []byte marshal
// as an empty JSON string for the former, while the latter as a JSON null.
// The non-zero values of those types always marshal as JSON strings.
//
// Related issues:
//	https://golang.org/issue/27589
//	https://golang.org/issue/37711
func TestNilSlicesAndMaps(t *testing.T) {
	type Composites struct {
		B []byte            // always serialized in v2 as a JSON string
		S []string          // always serialized in v2 as a JSON array
		M map[string]string // always serialized in v2 as a JSON object
	}

	for _, json := range jsonPackages {
		t.Run(path.Join("Marshal", json.Version), func(t *testing.T) {
			in := []Composites{
				{B: []byte(nil), S: []string(nil), M: map[string]string(nil)},
				{B: []byte{}, S: []string{}, M: map[string]string{}},
			}
			want := map[string]string{
				"v1": `[{"B":null,"S":null,"M":null},{"B":"","S":[],"M":{}}]`,
				"v2": `[{"B":"","S":[],"M":{}},{"B":"","S":[],"M":{}}]`, // v2 emits nil slices and maps as empty JSON objects and arrays
			}[json.Version]
			got, err := json.Marshal(in)
			if err != nil {
				t.Fatalf("json.Marshal error: %v", err)
			}
			if string(got) != want {
				t.Fatalf("json.Marshal = %s, want %s", got, want)
			}
		})
	}
}

// CallCheck implements json.{Marshaler,Unmarshaler} on a pointer receiver.
type CallCheck string

// MarshalJSON always returns a JSON string with the literal "CALLED".
func (*CallCheck) MarshalJSON() ([]byte, error) {
	return []byte(`"CALLED"`), nil
}

// UnmarshalJSON always stores a string with the literal "CALLED".
func (v *CallCheck) UnmarshalJSON([]byte) error {
	*v = `CALLED`
	return nil
}

// In v1, the implementation is inconsistent about whether it calls
// MarshalJSON and UnmarshalJSON methods declared on pointer receivers
// when it has an unaddressable value (per reflect.Value.CanAddr).
// When marshaling, it never boxes the value on the heap to make it addressable,
// while it sometimes boxes values (e.g., for map entries) when unmarshaling.
//
// In v2, the implementation always calls MarshalJSON and UnmarshalJSON methods
// by boxing the value on the heap if necessary.
//
// The v1 behavior is surprising at best and buggy at worst.
// Unfortunately, it cannot be changed without breaking existing usages.
func TestPointerReceiver(t *testing.T) {
	type Values struct {
		A [1]CallCheck
		S []CallCheck
		M map[string]CallCheck
		V CallCheck
		I any
	}

	for _, json := range jsonPackages {
		t.Run(path.Join("Marshal", json.Version), func(t *testing.T) {
			var cc CallCheck
			in := Values{
				A: [1]CallCheck{cc}, // MarshalJSON not called on v1
				S: []CallCheck{cc},
				M: map[string]CallCheck{"": cc}, // MarshalJSON not called on v1
				V: cc,                           // MarshalJSON not called on v1
				I: cc,                           // MarshalJSON not called on v1
			}
			want := map[string]string{
				"v1": `{"A":[""],"S":["CALLED"],"M":{"":""},"V":"","I":""}`,
				"v2": `{"A":["CALLED"],"S":["CALLED"],"M":{"":"CALLED"},"V":"CALLED","I":"CALLED"}`,
			}[json.Version]
			got, err := json.Marshal(in)
			if err != nil {
				t.Fatalf("json.Marshal error: %v", err)
			}
			if string(got) != want {
				t.Fatalf("json.Marshal = %s, want %s", got, want)
			}
		})
	}

	for _, json := range jsonPackages {
		t.Run(path.Join("Unmarshal", json.Version), func(t *testing.T) {
			in := `{"A":[""],"S":[""],"M":{"":""},"V":"","I":""}`
			called := CallCheck("CALLED") // resulting state if UnmarshalJSON is called
			want := map[string]Values{
				"v1": {
					A: [1]CallCheck{called},
					S: []CallCheck{called},
					M: map[string]CallCheck{"": called},
					V: called,
					I: "", // UnmarshalJSON not called on v1; replaced with Go string
				},
				"v2": {
					A: [1]CallCheck{called},
					S: []CallCheck{called},
					M: map[string]CallCheck{"": called},
					V: called,
					I: called,
				},
			}[json.Version]
			got := Values{
				A: [1]CallCheck{CallCheck("")},
				S: []CallCheck{CallCheck("")},
				M: map[string]CallCheck{"": CallCheck("")},
				V: CallCheck(""),
				I: CallCheck(""),
			}
			if err := json.Unmarshal([]byte(in), &got); err != nil {
				t.Fatalf("json.Unmarshal error: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("json.Unmarshal = %v, want %v", got, want)
			}
		})
	}
}

// In v1, maps are serialized in a deterministic order.
// In v2, maps are serialized in a non-deterministic order.
//
// The reason for the change is that v2 prioritizes performance and
// the guarantee that marshaling operates primarily in a streaming manner.
// The v2 API provides RawValue.Canonicalize if stability is needed.
//
// Related issue: https://golang.org/issue/33714
func TestMapDeterminism(t *testing.T) {
	const iterations = 10
	in := map[int]int{0: 0, 1: 1, 2: 2, 3: 3, 4: 4, 5: 5, 6: 6, 7: 7, 8: 8, 9: 9}

	for _, json := range jsonPackages {
		t.Run(path.Join("Marshal", json.Version), func(t *testing.T) {
			outs := make(map[string]bool)
			for i := 0; i < iterations; i++ {
				b, err := json.Marshal(in)
				if err != nil {
					t.Fatalf("json.Marshal error: %v", err)
				}
				outs[string(b)] = true
			}
			switch {
			case json.Version == "v1" && len(outs) != 1:
				t.Fatalf("json.Marshal serialized to %d unique forms, expected 1", len(outs))
			case json.Version == "v2" && len(outs) == 1:
				t.Logf("json.Marshal serialized to 1 unique form by chance; are you feeling lucky?")
			}
		})
	}

	t.Run("Marshal/v2/Canonicalize", func(t *testing.T) {
		want := `{"0":0,"1":1,"2":2,"3":3,"4":4,"5":5,"6":6,"7":7,"8":8,"9":9}`
		got, err := jsonv2.Marshal(in)
		if err != nil {
			t.Fatalf("json.Marshal error: %v", err)
		}
		if err := (*jsonv2.RawValue)(&got).Canonicalize(); err != nil {
			t.Fatalf("RawValue.Canonicalize error: %v", err)
		}
		if string(got) != want {
			t.Fatalf("RawValue.Canonicalize = %s, want %s", got, want)
		}
	})
}
