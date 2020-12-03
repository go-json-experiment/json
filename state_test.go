// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"fmt"
	"reflect"
	"testing"
)

func TestStateMachine(t *testing.T) {
	// To test a state machine, we pass an ordered sequence of operations and
	// check whether the current state is as expected.
	// The operation type is a union type of various possible operations,
	// which either call mutating methods on the state machine or
	// call accessor methods on state machine and verify the results.
	type operation interface{}
	type (
		// stackLengths checks the results of stateEntry.length accessors.
		stackLengths []int

		// appendTokens is sequence of token kinds to append where
		// none of them are expected to fail.
		//
		// For example: `[nft]` is equivalent to the following sequence:
		//
		//	pushArray()
		//	appendLiteral()
		//	appendString()
		//	appendNumber()
		//	popArray()
		//
		appendTokens string

		// appendToken is a single token kind to append with the expected error.
		appendToken struct {
			kind Kind
			want error
		}

		// needDelim checks the result of the needDelim accessor.
		needDelim struct {
			next Kind
			want byte
		}
	)

	// Each entry is a sequence of tokens to pass to the state machine.
	tests := []struct {
		label string
		ops   []operation
	}{{
		"TopLevelValues",
		[]operation{
			stackLengths{0},
			needDelim{'n', 0},
			appendTokens(`nft`),
			stackLengths{3},
			needDelim{'"', 0},
			appendTokens(`"0[]{}`),
			stackLengths{7},
		},
	}, {
		"ArrayValues",
		[]operation{
			stackLengths{0},
			needDelim{'[', 0},
			appendTokens(`[`),
			stackLengths{1, 0},
			needDelim{'n', 0},
			appendTokens(`nft`),
			stackLengths{1, 3},
			needDelim{'"', ','},
			appendTokens(`"0[]{}`),
			stackLengths{1, 7},
			needDelim{']', 0},
			appendTokens(`]`),
			stackLengths{1},
		},
	}, {
		"ObjectValues",
		[]operation{
			stackLengths{0},
			needDelim{'{', 0},
			appendTokens(`{`),
			// depthLength{2, 0},
			needDelim{'"', 0},
			appendTokens(`"`),
			// depthLength{2, 1},
			needDelim{'n', ':'},
			appendTokens(`n`),
			// depthLength{2, 2},
			needDelim{'"', ','},
			appendTokens(`"f"t`),
			// depthLength{2, 6},
			appendTokens(`"""0"[]"{}`),
			// depthLength{2, 14},
			needDelim{'}', 0},
			appendTokens(`}`),
			stackLengths{1},
		},
	}, {
		"ObjectCardinality",
		[]operation{
			appendTokens(`{`),

			// Appending any kind other than string for object name is an error.
			appendToken{'n', errMissingName},
			appendToken{'f', errMissingName},
			appendToken{'t', errMissingName},
			appendToken{'0', errMissingName},
			appendToken{'{', errMissingName},
			appendToken{'[', errMissingName},
			appendTokens(`"`),

			// Appending '}' without first appending any value is an error.
			appendToken{'}', errMissingValue},
			appendTokens(`"`),

			appendTokens(`}`),
		},
	}, {
		"MismatchingDelims",
		[]operation{
			appendToken{'}', errMismatchDelim}, // appending '}' without preceding '{'
			appendTokens(`[[{`),
			appendToken{']', errMismatchDelim}, // appending ']' that mismatches preceding '{'
			appendTokens(`}]`),
			appendToken{'}', errMismatchDelim}, // appending '}' that mismatches preceding '['
			appendTokens(`]`),
			appendToken{']', errMismatchDelim}, // appending ']' without precdeding '['
		},
	}}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			// Flatten appendTokens to sequence of appendToken entries.
			var ops []operation
			for _, op := range tt.ops {
				if toks, ok := op.(appendTokens); ok {
					for _, k := range []byte(toks) {
						ops = append(ops, appendToken{Kind(k), nil})
					}
					continue
				}
				ops = append(ops, op)
			}

			// Append each token to the state machine and check the output.
			var state stateMachine
			state.init()
			var sequence []Kind
			for _, op := range ops {
				switch op := op.(type) {
				case stackLengths:
					var got []int
					for _, e := range state {
						got = append(got, e.length())
					}
					want := []int(op)
					if !reflect.DeepEqual(got, want) {
						t.Errorf("%s: stack lengths mismatch:\ngot  %v\nwant %v", sequence, got, want)
					}
				case appendToken:
					got := state.append(op.kind)
					if !reflect.DeepEqual(got, op.want) {
						t.Errorf("%s: append('%c') = %v, want %v", sequence, op.kind, got, op.want)
					}
					if got == nil {
						sequence = append(sequence, op.kind)
					}
				case needDelim:
					if got := state.needDelim(op.next); got != op.want {
						t.Errorf("%s: needDelim('%c') = '%c', want '%c'", sequence, op.next, got, op.want)
					}
				default:
					panic(fmt.Sprintf("unknown operation: %T", op))
				}
			}
		})
	}
}

// append is a thin wrapper over the other append, pop, or push methods
// based on the token kind.
func (s *stateMachine) append(k Kind) error {
	switch k {
	case 'n', 'f', 't':
		return s.appendLiteral()
	case '"':
		return s.appendString()
	case '0':
		return s.appendNumber()
	case '{':
		return s.pushObject()
	case '}':
		return s.popObject()
	case '[':
		return s.pushArray()
	case ']':
		return s.popArray()
	default:
		panic(fmt.Sprintf("invalid token kind: '%c'", k))
	}
}
