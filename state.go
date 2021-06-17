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

type state struct {
	// tokens validates whether the next token kind is valid.
	tokens stateMachine

	// tokensBootstrap allows the tokens slice to not incur an extra
	// allocation for values with relatively small depth.
	tokensBootstrap [8]stateEntry

	// namespaces is a stack of object namespaces.
	namespaces objectNamespaceStack
}

func (s *state) init() {
	s.tokens.init(s.tokensBootstrap[:0])
}

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
func (m *stateMachine) init(bootstrap []stateEntry) {
	*m = append(bootstrap, stateTypeArray)
}

// depth is the current nested depth of JSON objects and arrays.
// It is one-indexed (i.e., top-level values have a depth of 1).
func (m stateMachine) depth() int {
	return len(m)
}

// depthLength reports the current nested depth and
// the length of the last JSON object or array.
func (m stateMachine) depthLength() (int, int) {
	return len(m), m[len(m)-1].length()
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

// needIndent reports whether indent whitespace should be injected.
// A zero value means that no whitespace should be injected.
// A positive value means '\n', indentPrefix, and (n-1) copies of indentBody
// should be appended to the output immediately before the next token.
func (m stateMachine) needIndent(next Kind) (n int) {
	willEnd := next == '}' || next == ']'
	switch e := m.last(); {
	case m.depth() == 1:
		return 0 // top-level values are never indented
	case e.length() == 0 && willEnd:
		return 0 // an empty object or array is never indented
	case e.length() == 0 || e.needImplicitComma(next):
		return m.depth()
	case willEnd:
		return m.depth() - 1
	default:
		return 0
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

// objectNamespaceStack is a stack of object namespaces.
// This data structure assists in detecting duplicate names.
type objectNamespaceStack []objectNamespace

// push starts a new namespace for a nested JSON object.
func (nss *objectNamespaceStack) push() {
	if cap(*nss) > len(*nss) {
		*nss = (*nss)[:len(*nss)+1]
	} else {
		*nss = append(*nss, objectNamespace{})
	}
}

// last returns a pointer to the last JSON object namespace.
func (nss objectNamespaceStack) last() *objectNamespace {
	return &nss[len(nss)-1]
}

// pop terminates the namespace for a nested JSON object.
func (nss *objectNamespaceStack) pop() {
	nss.last().reset()
	*nss = (*nss)[:len(*nss)-1]
}

// objectNamespace is the namespace for a JSON object.
// It relies on a linear search over all the names before switching
// to use a Go map for direct lookup.
//
// The zero value is an empty namespace ready for use.
type objectNamespace struct {
	// endOffsets is a list of offsets to the end of each name in buffers.
	// The length of offsets is the number of names in the namespace.
	endOffsets []uint
	// allNames is a back-to-back concatenation of every name in the namespace.
	allNames []byte
	// mapNames is a Go map containing every name in the namespace.
	// Only valid if non-nil.
	mapNames map[string]struct{}
}

// reset resets the namespace to be empty.
func (ns *objectNamespace) reset() {
	ns.endOffsets = ns.endOffsets[:0]
	ns.allNames = ns.allNames[:0]
	ns.mapNames = nil
	if cap(ns.endOffsets) > 1<<6 {
		ns.endOffsets = nil // avoid pinning arbitrarily large amounts of memory
	}
	if cap(ns.allNames) > 1<<10 {
		ns.allNames = nil // avoid pinning arbitrarily large amounts of memory
	}
}

// length reports the number names in the namespace.
func (ns *objectNamespace) length() int {
	return len(ns.endOffsets)
}

// get retrieves the ith name in the namespace.
func (ns *objectNamespace) get(i int) []byte {
	if i == 0 {
		return ns.allNames[:ns.endOffsets[0]]
	} else {
		return ns.allNames[ns.endOffsets[i-1]:ns.endOffsets[i-0]]
	}
}

// last retrieves the last name in the namespace.
func (ns *objectNamespace) last() []byte {
	return ns.get(ns.length() - 1)
}

// insert inserts an escaped name and reports whether it was inserted,
// which only occurs if name is not already in the namespace.
// The provided name must be a valid JSON string.
func (ns *objectNamespace) insert(b []byte) bool {
	// TODO: Consider making two variations of insert that operate on
	// both escaped and unescaped strings.
	allNames, _ := unescapeString(ns.allNames, b)
	name := allNames[len(ns.allNames):]

	// Switch to a map if the buffer is too large for linear search.
	// This does not add the current name to the map.
	if ns.length() > 16 {
		ns.mapNames = make(map[string]struct{})
		var startOffset uint
		for _, endOffset := range ns.endOffsets {
			name := ns.allNames[startOffset:endOffset]
			ns.mapNames[string(name)] = struct{}{}
			startOffset = endOffset
		}
	}

	if ns.mapNames == nil {
		// Perform linear search over the buffer to find matching names.
		// It provides O(n) lookup, but doesn't require any allocations.
		var startOffset uint
		for _, endOffset := range ns.endOffsets {
			if string(ns.allNames[startOffset:endOffset]) == string(name) {
				return false
			}
			startOffset = endOffset
		}
	} else {
		// Use the map if it is populated.
		// It provides O(1) lookup, but requires a string allocation per name.
		if _, ok := ns.mapNames[string(name)]; ok {
			return false
		}
		ns.mapNames[string(name)] = struct{}{}
	}

	ns.allNames = allNames
	ns.endOffsets = append(ns.endOffsets, uint(len(ns.allNames)))
	return true
}

// removeLast removes the last name in the namespace.
func (ns *objectNamespace) removeLast() {
	// TODO: Delete this if Marshal/Unmarshal don't need to provide the ability
	// to unwrite/unread an object name.
	if ns.mapNames != nil {
		delete(ns.mapNames, string(ns.last()))
	}
	if ns.length()-1 == 0 {
		ns.endOffsets = ns.endOffsets[:0]
		ns.allNames = ns.allNames[:0]
	} else {
		ns.endOffsets = ns.endOffsets[:ns.length()-1]
		ns.allNames = ns.allNames[:ns.endOffsets[ns.length()-1]]
	}
}
