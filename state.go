// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

var (
	errMissingName   = &SyntaxError{str: "missing string for object name"}
	errMissingColon  = &SyntaxError{str: "missing character ':' after object name"}
	errMissingValue  = &SyntaxError{str: "missing value after object name"}
	errMissingComma  = &SyntaxError{str: "missing character ',' after object or array value"}
	errMismatchDelim = &SyntaxError{str: "mismatching structural token for object or array"}
)

// stateMachine is a push-down automaton that validates whether
// a sequence of tokens is valid or not according to the JSON grammar.
// It is useful for both encoding and decoding.
//
// It is a stack where each entry represents a nested JSON object or array.
// The stack has a minimum depth of 1 where the first level is a
// virtual JSON array to handle a stream of top-level JSON values.
// The top-level virtual JSON array is special in that it doesn't require commas
// between each JSON value.
//
// For performance, most methods are carefully written to be inlineable.
// The zero value is not a valid state machine; call init first.
type stateMachine []stateEntry

// init initializes the state machine.
// The machine always starts with a minimum depth of 1.
func (m *stateMachine) init() {
	*m = append((*m)[:0], stateTypeArray)
}

// depth is the current nested depth of JSON objects and arrays.
// It is one-indexed (i.e., top-level values have a depth of 1).
func (m stateMachine) depth() int {
	return len(m)
}

// last returns a pointer to the last entry.
func (m stateMachine) last() *stateEntry {
	return &m[len(m)-1]
}

// appendLiteral appends a JSON literal as the next token in the sequence.
// If an error is returned, the state is not mutated.
func (m stateMachine) appendLiteral() error {
	switch e := m.last(); {
	case e.needObjectName():
		return errMissingName
	default:
		e.increment()
		return nil
	}
}

// appendString appends a JSON string as the next token in the sequence.
// If an error is returned, the state is not mutated.
func (m stateMachine) appendString() error {
	switch e := m.last(); {
	default:
		e.increment()
		return nil
	}
}

// appendNumber appends a JSON number as the next token in the sequence.
// If an error is returned, the state is not mutated.
func (m stateMachine) appendNumber() error {
	return m.appendLiteral()
}

// pushObject appends a JSON start object token as next in the sequence.
// If an error is returned, the state is not mutated.
func (m *stateMachine) pushObject() error {
	switch e := m.last(); {
	case e.needObjectName():
		return errMissingName
	default:
		e.increment()
		*m = append(*m, stateTypeObject)
		return nil
	}
}

// popObject appends a JSON end object token as next in the sequence.
// If an error is returned, the state is not mutated.
func (m *stateMachine) popObject() error {
	switch e := m.last(); {
	case !e.isObject():
		return errMismatchDelim
	case e.needObjectValue():
		return errMissingValue
	default:
		*m = (*m)[:len(*m)-1]
		return nil
	}
}

// pushArray appends a JSON start array token as next in the sequence.
// If an error is returned, the state is not mutated.
func (m *stateMachine) pushArray() error {
	switch e := m.last(); {
	case e.needObjectName():
		return errMissingName
	default:
		e.increment()
		*m = append(*m, stateTypeArray)
		return nil
	}
}

// popArray appends a JSON end array token as next in the sequence.
// If an error is returned, the state is not mutated.
func (m *stateMachine) popArray() error {
	switch e := m.last(); {
	case !e.isArray() || len(*m) == 1: // forbid popping top-level virtual JSON array
		return errMismatchDelim
	default:
		*m = (*m)[:len(*m)-1]
		return nil
	}
}

// needDelim reports whether a colon or comma token should be implicitly emitted
// before the next token of the specified kind.
// A zero value means no delimiter should be emitted.
func (m stateMachine) needDelim(next Kind) (delim byte) {
	switch e := m.last(); {
	case e.needImplicitColon():
		return ':'
	case e.needImplicitComma(next) && len(m) != 1: // comma not needed for top-level values
		return ','
	}
	return 0
}

// checkDelim reports whether the specified delimiter should be there given
// the kind of the next token that appears immediately afterwards.
func (m stateMachine) checkDelim(delim byte, next Kind) error {
	switch needDelim := m.needDelim(next); {
	case needDelim == delim:
		return nil
	case needDelim == ':':
		return errMissingColon
	case needDelim == ',':
		return errMissingComma
	default:
		return newInvalidCharacterError(delim, "before next token")
	}
}

// stateEntry encodes several artifacts within a single unsigned integer:
//	• whether this represents a JSON object or array and
//	• how many elements are in this JSON object or array.
type stateEntry uint64

const (
	// The type mask (1 bit) records whether this is a JSON object or array.
	stateTypeMask   stateEntry = 0x8000_0000_0000_0000
	stateTypeObject stateEntry = 0x8000_0000_0000_0000
	stateTypeArray  stateEntry = 0x0000_0000_0000_0000

	// The count mask (63 bits) records the number of elements.
	stateCountMask    stateEntry = 0x7fff_ffff_ffff_ffff
	stateCountLSBMask stateEntry = 0x0000_0000_0000_0001
	stateCountOdd     stateEntry = 0x0000_0000_0000_0001
	stateCountEven    stateEntry = 0x0000_0000_0000_0000
)

// length reports the number of elements in the JSON object or array.
// Each name and value in an object entry is treated as a separate element.
func (e stateEntry) length() int {
	return int(e & stateCountMask)
}

// isObject reports whether this is a JSON object.
func (e stateEntry) isObject() bool {
	return e&stateTypeMask == stateTypeObject
}

// isArray reports whether this is a JSON array.
func (e stateEntry) isArray() bool {
	return e&stateTypeMask == stateTypeArray
}

// needObjectName reports whether the next token must be a JSON string,
// which is necessary for JSON object names.
func (e stateEntry) needObjectName() bool {
	return e&(stateTypeMask|stateCountLSBMask) == stateTypeObject|stateCountEven
}

// needImplicitColon reports whether an impicit colon should occur next,
// which always occurs after JSON object names.
func (e stateEntry) needImplicitColon() bool {
	return e.needObjectValue()
}

// needObjectValue reports whether the next token must be a JSON value,
// which is necessary after every JSON object name.
func (e stateEntry) needObjectValue() bool {
	return e&(stateTypeMask|stateCountLSBMask) == stateTypeObject|stateCountOdd
}

// needImplicitComma reports whether an impicit comma should occur next,
// which always occurs after a value in a JSON object or array
// before the next value (or name).
func (e stateEntry) needImplicitComma(next Kind) bool {
	return !e.needObjectValue() && e.length() > 0 && next != '}' && next != ']'
}

// increment increments the number of elements for the current object or array.
// This assumes that overflow won't practically be an issue since
// 1<<bits.OnesCount(stateCountMask) is sufficiently large.
func (e *stateEntry) increment() {
	(*e)++
}
