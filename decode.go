// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"io"
	"math"
	"strconv"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// NOTE: The logic for decoding is complicated by the fact that reading from
// an io.Reader into a temporary buffer means that the buffer may contain a
// truncated portion of some valid input, requiring the need to fetch more data.
//
// This file is structured in the following way:
//
//	• consumeXXX functions parse a JSON token from a []byte.
//	  If the buffer appears truncated, then it returns io.ErrUnexpectedEOF.
//	  The consumeSimpleXXX functions are so named because they only handle
//	  a subset of the grammar for the JSON token being parsed.
//	  They do not handle the full grammar to keep these functions inlineable.
//
//	• Decoder.consumeXXX methods parse a JSON token from the internal buffer,
//	  automatically fetching more input if necessary. These methods take
//	  a position relative to the start of Decoder.buf as an argument and
//	  return the end of the consumed JSON token as a position,
//	  also relative to the start of Decoder.buf.
//
//	• In the event of an I/O errors or state machine violations,
//	  the implementation avoids mutating the state of Decoder
//	  (aside from the book-keeping needed to implement Decoder.fetch).
//	  For this reason, only Decoder.ReadToken and Decoder.ReadValue are
//	  responsible for updated Decoder.prevStart and Decoder.prevEnd.
//
//	• For performance, much of the implementation uses the pattern of calling
//	  the inlineable consumeXXX functions first, and if more work is neccessary,
//	  then it calls the slower Decoder.consumeXXX methods.
//	  TODO: Revisit this pattern if the Go compiler provides finer control
//	  over exactly which calls are inlined or not.

// DecodeOptions configures how JSON decoding operates.
// The zero value is equivalent to the default settings,
// which is compliant with RFC 8259.
type DecodeOptions struct {
	// TODO: Rename as UnmarshalOptions?

	// RejectDuplicateNames specifies that JSON objects must not contain
	// duplicate member names to ensure that the input is compliant with
	// RFC 7493, section 2.3. Use of the feature incurs some performance and
	// memory cost needed to keep track of all member names processed so far.
	RejectDuplicateNames bool

	// AllowInvalidUTF8 specifies that JSON strings may contain invalid UTF-8,
	// which will be mangled as the Unicode replacement character, U+FFFD.
	// This causes the decoder to break compliance with
	// RFC 7493, section 2.1, and RFC 8259, section 8.1.
	AllowInvalidUTF8 bool
}

// Decoder is a streaming decoder for raw JSON values and tokens.
// It is used to read a stream of top-level JSON values,
// each separated by optional whitespace characters.
//
// ReadToken and ReadValue calls may be interleaved.
// For example, the following JSON value:
//
//	{"name":"value","array":[null,false,true,3.14159],"object":{"k":"v"}}
//
// can be parsed with the following calls (ignoring errors for brevity):
//
//	d.ReadToken() // {
//	d.ReadToken() // "name"
//	d.ReadToken() // "value"
//	d.ReadValue() // "array"
//	d.ReadToken() // [
//	d.ReadToken() // null
//	d.ReadToken() // false
//	d.ReadValue() // true
//	d.ReadToken() // 3.14159
//	d.ReadToken() // ]
//	d.ReadValue() // "object"
//	d.ReadValue() // {"k":"v"}
//	d.ReadToken() // }
//
// The above is one of many possible sequence of calls and
// may not represent the most sensible method to call for any given token/value.
// For example, it is probably more common to call ReadToken to obtain a
// string token for object names.
type Decoder struct {
	state
	decodeBuffer
	options DecodeOptions
}

// decodeBuffer is a buffer split into 4 segments:
//
//	• buf[0:prevEnd]         // already read portion of the buffer
//	• buf[prevStart:prevEnd] // previously read value
//	• buf[prevEnd:len(buf)]  // unread portion of the buffer
//	• buf[len(buf):cap(buf)] // unused portion of the buffer
//
// Invariants:
//	0 ≤ prevStart ≤ prevEnd ≤ len(buf) ≤ cap(buf)
type decodeBuffer struct {
	buf       []byte
	prevStart int
	prevEnd   int

	// baseOffset can be added to prevStart and prevEnd to obtain
	// the absolute offset relative to the start of io.Reader stream.
	baseOffset int64

	rd io.Reader
}

// NewDecoder constructs a new streaming decoder reading from r.
func NewDecoder(r io.Reader) *Decoder {
	return DecodeOptions{}.NewDecoder(r)
}

// NewDecoder constructs a new streaming decoder reading from r
// configured with the provided options.
func (o DecodeOptions) NewDecoder(r io.Reader) *Decoder {
	if r == nil {
		panic("json: invalid nil io.Reader")
	}
	return o.newDecoder(r, nil)
}
func (o DecodeOptions) newDecoder(r io.Reader, b []byte) *Decoder {
	d := new(Decoder)
	d.state.init()
	d.rd = r
	d.buf = b
	d.options = o
	return d
}

// fetch reads at least 1 byte from the underlying io.Reader.
// It returns io.ErrUnexpectedEOF if zero bytes were read and io.EOF was seen.
func (d *decodeBuffer) fetch() error {
	if d.rd == nil {
		return io.ErrUnexpectedEOF
	}

	// Allocate initial buffer if empty.
	if cap(d.buf) == 0 {
		d.buf = make([]byte, 0, 64)
	}

	// Check whether to grow the buffer.
	const maxBufferSize = 4 << 10
	const growthSizeFactor = 2 // higher value is faster
	const growthRateFactor = 2 // higher value is slower
	// By default, grow if below the maximum buffer size.
	grow := cap(d.buf) <= maxBufferSize/growthSizeFactor
	// Growing can be expensive, so only grow
	// if a sufficient number of bytes have been processed.
	grow = grow && int64(cap(d.buf)/growthRateFactor) > d.previousOffsetEnd()
	// If prevStart==0, then fetch was called in order to fetch more data
	// to finish consuming a large JSON value contiguously.
	// Grow if less than 25% of the remaining capacity is available.
	// Note that this may cause the input buffer to exceed maxBufferSize.
	grow = grow || (d.prevStart == 0 && len(d.buf) >= 3*cap(d.buf)/4)

	// Move unread portion of the data to the front.
	if grow {
		// TODO: Provide a hard limit on the maximum internal buffer size?
		buf := make([]byte, 0, cap(d.buf)*growthSizeFactor)
		d.buf = append(buf, d.buf[d.prevStart:]...)
	} else {
		n := copy(d.buf[:cap(d.buf)], d.buf[d.prevStart:])
		d.buf = d.buf[:n]
	}
	d.baseOffset += int64(d.prevStart)
	d.prevEnd -= d.prevStart
	d.prevStart = 0

	// Read more data into the internal buffer.
	for {
		n, err := d.rd.Read(d.buf[len(d.buf):cap(d.buf)])
		switch {
		case n > 0:
			d.buf = d.buf[:len(d.buf)+n]
			return nil // ignore errors if any bytes are read
		case err == io.EOF:
			return io.ErrUnexpectedEOF
		case err != nil:
			return &wrapError{"read error", err}
		default:
			continue // Read returned (0, nil)
		}
	}
}

// invalidatePreviousRead invalidates buffers returned by ReadValue calls
// so that the first byte is an invalid character.
// This Hyrum-proofs the API against faulty application code that assumes
// values returned by ReadValue remain valid past subsequent Read calls.
func (d *decodeBuffer) invalidatePreviousRead() {
	// Avoid mutating the buffer if d.rd is nil which implies that d.buf
	// is provided by the user code and may not expect mutations.
	if d.rd != nil && d.prevStart < d.prevEnd {
		d.buf[d.prevStart] = '#' // invalid character outside of JSON string
		d.prevStart = d.prevEnd
	}
}

// needMore reports whether there are no more unread bytes.
func (d *decodeBuffer) needMore(pos int) bool {
	// NOTE: The arguments and logic are kept simple to keep this inlineable.
	return pos == len(d.buf)
}

// injectSyntaxErrorWithPosition wraps a SyntaxError with the position,
// otherwise it returns the error as is.
// It takes a position relative to the start of the start of d.buf.
func (d *decodeBuffer) injectSyntaxErrorWithPosition(err error, pos int) error {
	if serr, ok := err.(*SyntaxError); ok {
		return serr.withOffset(d.baseOffset + int64(pos))
	}
	return err
}

func (d *decodeBuffer) previousOffsetStart() int64 { return d.baseOffset + int64(d.prevStart) }
func (d *decodeBuffer) previousOffsetEnd() int64   { return d.baseOffset + int64(d.prevEnd) }
func (d *decodeBuffer) previousBuffer() []byte     { return d.buf[d.prevStart:d.prevEnd] }
func (d *decodeBuffer) unreadBuffer() []byte       { return d.buf[d.prevEnd:len(d.buf)] }

// PeekKind retrieves the next token kind, but does not advance the read offset.
// It returns 0 if there are no more tokens.
func (d *Decoder) PeekKind() Kind {
	// TODO: Do we want to distinguish between io.EOF and other errors?
	//
	// One reason to return a non-zero value is because people may check
	// d.PeekKind() > 0 as a way to determine whether there are more tokens.
	// Returning non-zero likely causes the user code to perform subsequent
	// operations on Decoder, which would return an error. In contrast,
	// a zero value might give the illusion that the token stream properly ended
	// with io.EOF and the user code will silently ignore any errors.
	d.invalidatePreviousRead()

	var err error
	pos := d.prevEnd

	// Consume leading whitespace.
	pos += consumeWhitespace(d.buf[pos:])
	if d.needMore(pos) {
		if pos, err = d.consumeWhitespace(pos); err != nil {
			return invalidKind
		}
	}

	// Consume colon or comma.
	if c := d.buf[pos]; c == ':' || c == ',' {
		pos += 1
		pos += consumeWhitespace(d.buf[pos:])
		if d.needMore(pos) {
			if pos, err = d.consumeWhitespace(pos); err != nil {
				return invalidKind
			}
		}
	}

	return Kind(d.buf[pos]).normalize()
}

// ReadToken reads the next Token, advancing the read offset.
// The returned token is only valid until the next Peek or Read call.
// It returns io.EOF if there are no more tokens.
func (d *Decoder) ReadToken() (Token, error) {
	d.invalidatePreviousRead()

	var err error
	pos := d.prevEnd

	// Consume leading whitespace.
	pos += consumeWhitespace(d.buf[pos:])
	if d.needMore(pos) {
		if pos, err = d.consumeWhitespace(pos); err != nil {
			if err == io.ErrUnexpectedEOF && d.tokens.depth() == 1 {
				err = io.EOF // EOF possibly if no Tokens present after top-level value
			}
			return Token{}, err
		}
	}

	// Consume colon or comma.
	var delim byte
	if c := d.buf[pos]; c == ':' || c == ',' {
		delim = c
		pos += 1
		pos += consumeWhitespace(d.buf[pos:])
		if d.needMore(pos) {
			if pos, err = d.consumeWhitespace(pos); err != nil {
				return Token{}, err
			}
		}
	}
	next := Kind(d.buf[pos]).normalize()
	if d.tokens.needDelim(next) != delim {
		pos = d.prevEnd // restore position to right after leading whitespace
		pos += consumeWhitespace(d.buf[pos:])
		err = d.tokens.checkDelim(delim, next)
		return Token{}, d.injectSyntaxErrorWithPosition(err, pos)
	}

	// Handle the next token.
	var n int
	switch next {
	case 'n':
		if consumeNull(d.buf[pos:]) == 0 {
			pos, err = d.consumeLiteral(pos, "null")
			if err != nil {
				return Token{}, d.injectSyntaxErrorWithPosition(err, pos)
			}
		} else {
			pos += len("null")
		}
		if err = d.tokens.appendLiteral(); err != nil {
			return Token{}, d.injectSyntaxErrorWithPosition(err, pos-len("null")) // report position at start of literal
		}
		d.prevStart, d.prevEnd = pos, pos
		return Null, nil

	case 'f':
		if consumeFalse(d.buf[pos:]) == 0 {
			pos, err = d.consumeLiteral(pos, "false")
			if err != nil {
				return Token{}, d.injectSyntaxErrorWithPosition(err, pos)
			}
		} else {
			pos += len("false")
		}
		if err = d.tokens.appendLiteral(); err != nil {
			return Token{}, d.injectSyntaxErrorWithPosition(err, pos-len("false")) // report position at start of literal
		}
		d.prevStart, d.prevEnd = pos, pos
		return False, nil

	case 't':
		if consumeTrue(d.buf[pos:]) == 0 {
			pos, err = d.consumeLiteral(pos, "true")
			if err != nil {
				return Token{}, d.injectSyntaxErrorWithPosition(err, pos)
			}
		} else {
			pos += len("true")
		}
		if err = d.tokens.appendLiteral(); err != nil {
			return Token{}, d.injectSyntaxErrorWithPosition(err, pos-len("true")) // report position at start of literal
		}
		d.prevStart, d.prevEnd = pos, pos
		return True, nil

	case '"':
		if n = consumeSimpleString(d.buf[pos:]); n == 0 {
			oldAbsPos := d.baseOffset + int64(pos)
			pos, err = d.consumeString(pos)
			newAbsPos := d.baseOffset + int64(pos)
			n = int(newAbsPos - oldAbsPos)
			if err != nil {
				return Token{}, d.injectSyntaxErrorWithPosition(err, pos)
			}
		} else {
			pos += n
		}
		if d.options.RejectDuplicateNames && d.tokens.last().needObjectName() && !d.namespaces.last().insert(d.buf[pos-n:pos]) {
			err = &SyntaxError{str: "duplicate name " + string(d.buf[pos-n:pos]) + " in object"}
			return Token{}, d.injectSyntaxErrorWithPosition(err, pos-n) // report position at start of string
		}
		if err = d.tokens.appendString(); err != nil {
			return Token{}, d.injectSyntaxErrorWithPosition(err, pos-n) // report position at start of string
		}
		d.prevStart, d.prevEnd = pos-n, pos
		return Token{raw: &d.decodeBuffer, num: uint64(d.previousOffsetStart())}, nil

	case '0':
		// NOTE: Since JSON numbers are not self-terminating,
		// we need to make sure that the next byte is not part of a number.
		if n = consumeSimpleNumber(d.buf[pos:]); n == 0 || d.needMore(pos+n) {
			oldAbsPos := d.baseOffset + int64(pos)
			pos, err = d.consumeNumber(pos)
			newAbsPos := d.baseOffset + int64(pos)
			n = int(newAbsPos - oldAbsPos)
			if err != nil {
				return Token{}, d.injectSyntaxErrorWithPosition(err, pos)
			}
		} else {
			pos += n
		}
		if err = d.tokens.appendNumber(); err != nil {
			return Token{}, d.injectSyntaxErrorWithPosition(err, pos-n) // report position at start of number
		}
		d.prevStart, d.prevEnd = pos-n, pos
		return Token{raw: &d.decodeBuffer, num: uint64(d.previousOffsetStart())}, nil

	case '{':
		if err = d.tokens.pushObject(); err != nil {
			return Token{}, d.injectSyntaxErrorWithPosition(err, pos)
		}
		if d.options.RejectDuplicateNames {
			d.namespaces.push()
		}
		pos += 1
		d.prevStart, d.prevEnd = pos, pos
		return ObjectStart, nil

	case '}':
		if err = d.tokens.popObject(); err != nil {
			return Token{}, d.injectSyntaxErrorWithPosition(err, pos)
		}
		if d.options.RejectDuplicateNames {
			d.namespaces.pop()
		}
		pos += 1
		d.prevStart, d.prevEnd = pos, pos
		return ObjectEnd, nil

	case '[':
		if err = d.tokens.pushArray(); err != nil {
			return Token{}, d.injectSyntaxErrorWithPosition(err, pos)
		}
		pos += 1
		d.prevStart, d.prevEnd = pos, pos
		return ArrayStart, nil

	case ']':
		if err = d.tokens.popArray(); err != nil {
			return Token{}, d.injectSyntaxErrorWithPosition(err, pos)
		}
		pos += 1
		d.prevStart, d.prevEnd = pos, pos
		return ArrayEnd, nil

	default:
		err = newInvalidCharacterError(byte(next), "at start of token")
		return Token{}, d.injectSyntaxErrorWithPosition(err, pos)
	}
}

// ReadValue returns the next raw JSON value, advancing the read offset.
// The value is stripped of any leading or trailing whitespace.
// The returned value is only valid until the next Peek or Read call and
// may not be mutated while the Decoder remains in use.
// It returns io.EOF if there are no more values.
func (d *Decoder) ReadValue() (RawValue, error) {
	d.invalidatePreviousRead()

	var err error
	pos := d.prevEnd

	// Consume leading whitespace.
	pos += consumeWhitespace(d.buf[pos:])
	if d.needMore(pos) {
		if pos, err = d.consumeWhitespace(pos); err != nil {
			if err == io.ErrUnexpectedEOF && d.tokens.depth() == 1 {
				err = io.EOF // EOF possibly if no Tokens present after top-level value
			}
			return nil, err
		}
	}

	// Consume colon or comma.
	var delim byte
	if c := d.buf[pos]; c == ':' || c == ',' {
		delim = c
		pos += 1
		pos += consumeWhitespace(d.buf[pos:])
		if d.needMore(pos) {
			if pos, err = d.consumeWhitespace(pos); err != nil {
				return nil, err
			}
		}
	}
	next := Kind(d.buf[pos]).normalize()
	if d.tokens.needDelim(next) != delim {
		pos = d.prevEnd // restore position to right after leading whitespace
		pos += consumeWhitespace(d.buf[pos:])
		err = d.tokens.checkDelim(delim, next)
		return nil, d.injectSyntaxErrorWithPosition(err, pos)
	}

	// Handle the next value.
	oldAbsPos := d.baseOffset + int64(pos)
	pos, err = d.consumeValue(pos)
	newAbsPos := d.baseOffset + int64(pos)
	n := int(newAbsPos - oldAbsPos)
	if err != nil {
		return nil, d.injectSyntaxErrorWithPosition(err, pos)
	}
	switch next {
	case 'n', 't', 'f':
		err = d.tokens.appendLiteral()
	case '"':
		if d.options.RejectDuplicateNames && d.tokens.last().needObjectName() && !d.namespaces.last().insert(d.buf[pos-n:pos]) {
			err = &SyntaxError{str: "duplicate name " + string(d.buf[pos-n:pos]) + " in object"}
			break
		}
		err = d.tokens.appendString()
	case '0':
		err = d.tokens.appendNumber()
	case '{':
		if err = d.tokens.pushObject(); err != nil {
			break
		}
		if err = d.tokens.popObject(); err != nil {
			panic("BUG: popObject should never fail immediately after pushObject: " + err.Error())
		}
	case '[':
		if err = d.tokens.pushArray(); err != nil {
			break
		}
		if err = d.tokens.popArray(); err != nil {
			panic("BUG: popArray should never fail immediately after pushArray: " + err.Error())
		}
	}
	if err != nil {
		return nil, d.injectSyntaxErrorWithPosition(err, pos-n) // report position at start of value
	}
	d.prevEnd = pos
	d.prevStart = pos - n
	return d.buf[pos-n : pos : pos], nil
}

// checkEOF verifies that the input has no more data.
func (d *Decoder) checkEOF() error {
	switch pos, err := d.consumeWhitespace(d.prevEnd); err {
	case nil:
		return newInvalidCharacterError(d.buf[pos], "after top-level value")
	case io.ErrUnexpectedEOF:
		return nil
	default:
		return err
	}
}

// consumeWhitespace consumes all whitespace starting at d.buf[pos:].
// It returns the new position in d.buf immediately after the last whitespace.
// If it returns nil, there is guaranteed to at least be one unread byte.
//
// The following pattern is common in this implementation:
//
//	pos += consumeWhitespace(d.buf[pos:])
//	if d.needMore(pos) {
//		if pos, err = d.consumeWhitespace(pos); err != nil {
//			return ...
//		}
//	}
//
// It is difficult to simplify this without sacrificing performance since
// consumeWhitespace must be inlined. The body of the if statement is
// executed only in rare situations where we need to fetch more data.
// Since fetching may return an error, we also need to check the error.
func (d *Decoder) consumeWhitespace(pos int) (newPos int, err error) {
	for {
		pos += consumeWhitespace(d.buf[pos:])
		if d.needMore(pos) {
			absPos := d.baseOffset + int64(pos)
			err = d.fetch() // will mutate d.buf and invalidate pos
			pos = int(absPos - d.baseOffset)
			if err != nil {
				return pos, err
			}
			continue
		}
		return pos, nil
	}
}

// consumeValue consumes a single JSON value starting at d.buf[pos:].
// It returns the new position in d.buf immediately after the value.
func (d *Decoder) consumeValue(pos int) (newPos int, err error) {
	for {
		var n int
		var err error
		switch next := Kind(d.buf[pos]).normalize(); next {
		case 'n':
			if n = consumeNull(d.buf[pos:]); n == 0 {
				n, err = consumeLiteral(d.buf[pos:], "null")
			}
		case 'f':
			if n = consumeFalse(d.buf[pos:]); n == 0 {
				n, err = consumeLiteral(d.buf[pos:], "false")
			}
		case 't':
			if n = consumeTrue(d.buf[pos:]); n == 0 {
				n, err = consumeLiteral(d.buf[pos:], "true")
			}
		case '"':
			if n = consumeSimpleString(d.buf[pos:]); n == 0 {
				return d.consumeString(pos)
			}
		case '0':
			// NOTE: Since JSON numbers are not self-terminating,
			// we need to make sure that the next byte is not part of a number.
			if n = consumeSimpleNumber(d.buf[pos:]); n == 0 || d.needMore(pos+n) {
				return d.consumeNumber(pos)
			}
		case '{':
			return d.consumeObject(pos)
		case '[':
			return d.consumeArray(pos)
		default:
			return pos, newInvalidCharacterError(byte(next), "at start of value")
		}
		if err == io.ErrUnexpectedEOF {
			absPos := d.baseOffset + int64(pos)
			err = d.fetch() // will mutate d.buf and invalidate pos
			pos = int(absPos - d.baseOffset)
			if err != nil {
				return pos, err
			}
			continue
		}
		return pos + n, err
	}
}

// consumeLiteral consumes a single JSON literal starting at d.buf[pos:].
// It returns the new position in d.buf immediately after the literal.
func (d *Decoder) consumeLiteral(pos int, lit string) (newPos int, err error) {
	for {
		n, err := consumeLiteral(d.buf[pos:], lit)
		if err == io.ErrUnexpectedEOF {
			absPos := d.baseOffset + int64(pos)
			err = d.fetch() // will mutate d.buf and invalidate pos
			pos = int(absPos - d.baseOffset)
			if err != nil {
				return pos, err
			}
			continue
		}
		return pos + n, err
	}
}

// consumeString consumes a single JSON string starting at d.buf[pos:].
// It returns the new position in d.buf immediately after the string.
func (d *Decoder) consumeString(pos int) (newPos int, err error) {
	var n int
	for {
		n, err = consumeStringResumable(d.buf[pos:], n, !d.options.AllowInvalidUTF8)
		if err == io.ErrUnexpectedEOF {
			absPos := d.baseOffset + int64(pos)
			err = d.fetch() // will mutate d.buf and invalidate pos
			pos = int(absPos - d.baseOffset)
			if err != nil {
				return pos, err
			}
			continue
		}
		return pos + n, err
	}
}

// consumeNumber consumes a single JSON number starting at d.buf[pos:].
// It returns the new position in d.buf immediately after the number.
func (d *Decoder) consumeNumber(pos int) (newPos int, err error) {
	var n int
	var state consumeNumberState
	for {
		n, state, err = consumeNumberResumable(d.buf[pos:], n, state)
		// NOTE: Since JSON numbers are not self-terminating,
		// we need to make sure that the next byte is not part of a number.
		if err == io.ErrUnexpectedEOF || d.needMore(pos+n) {
			mayTerminate := err == nil
			absPos := d.baseOffset + int64(pos)
			err = d.fetch() // will mutate d.buf and invalidate pos
			pos = int(absPos - d.baseOffset)
			if err != nil {
				if mayTerminate && err == io.ErrUnexpectedEOF {
					return pos + n, nil
				}
				return pos, err
			}
			continue
		}
		return pos + n, err
	}
}

// consumeObject consumes a single JSON object starting at d.buf[pos:].
// It returns the new position in d.buf immediately after the object.
func (d *Decoder) consumeObject(pos int) (newPos int, err error) {
	var n int
	var names *objectNamespace
	if d.options.RejectDuplicateNames {
		d.namespaces.push()
		defer d.namespaces.pop()
		names = d.namespaces.last()
	}

	// Handle before start.
	if d.buf[pos] != '{' {
		panic("BUG: consumeObject must be called with a buffer that starts with '{'")
	}
	pos++

	// Handle after start.
	pos += consumeWhitespace(d.buf[pos:])
	if d.needMore(pos) {
		if pos, err = d.consumeWhitespace(pos); err != nil {
			return pos, err
		}
	}
	if d.buf[pos] == '}' {
		pos++
		return pos, nil
	}

	for {
		// Handle before name.
		pos += consumeWhitespace(d.buf[pos:])
		if d.needMore(pos) {
			if pos, err = d.consumeWhitespace(pos); err != nil {
				return pos, err
			}
		}
		if n = consumeSimpleString(d.buf[pos:]); n == 0 {
			oldAbsPos := d.baseOffset + int64(pos)
			pos, err = d.consumeString(pos)
			newAbsPos := d.baseOffset + int64(pos)
			n = int(newAbsPos - oldAbsPos)
			if err != nil {
				return pos, err
			}
		} else {
			pos += n
		}
		if d.options.RejectDuplicateNames && !names.insert(d.buf[pos-n:pos]) {
			return pos - n, &SyntaxError{str: "duplicate name " + string(d.buf[pos-n:pos]) + " in object"}
		}

		// Handle after name.
		pos += consumeWhitespace(d.buf[pos:])
		if d.needMore(pos) {
			if pos, err = d.consumeWhitespace(pos); err != nil {
				return pos, err
			}
		}
		if c := d.buf[pos]; c != ':' {
			return pos, newInvalidCharacterError(c, "after object name (expecting ':')")
		}
		pos++

		// Handle before value.
		pos += consumeWhitespace(d.buf[pos:])
		if d.needMore(pos) {
			if pos, err = d.consumeWhitespace(pos); err != nil {
				return pos, err
			}
		}
		pos, err = d.consumeValue(pos)
		if err != nil {
			return pos, err
		}

		// Handle after value.
		pos += consumeWhitespace(d.buf[pos:])
		if d.needMore(pos) {
			if pos, err = d.consumeWhitespace(pos); err != nil {
				return pos, err
			}
		}
		switch c := d.buf[pos]; c {
		case ',':
			pos++
			continue
		case '}':
			pos++
			return pos, nil
		default:
			return pos, newInvalidCharacterError(c, "after object value (expecting ',' or '}')")
		}
	}
}

// consumeArray consumes a single JSON array starting at d.buf[pos:].
// It returns the new position in d.buf immediately after the array.
func (d *Decoder) consumeArray(pos int) (newPos int, err error) {
	// Handle before start.
	if d.buf[pos] != '[' {
		panic("BUG: consumeArray must be called with a buffer that starts with '['")
	}
	pos++

	// Handle after start.
	pos += consumeWhitespace(d.buf[pos:])
	if d.needMore(pos) {
		if pos, err = d.consumeWhitespace(pos); err != nil {
			return pos, err
		}
	}
	if d.buf[pos] == ']' {
		pos++
		return pos, nil
	}

	for {
		// Handle before value.
		pos += consumeWhitespace(d.buf[pos:])
		if d.needMore(pos) {
			if pos, err = d.consumeWhitespace(pos); err != nil {
				return pos, err
			}
		}
		pos, err = d.consumeValue(pos)
		if err != nil {
			return pos, err
		}

		// Handle after value.
		pos += consumeWhitespace(d.buf[pos:])
		if d.needMore(pos) {
			if pos, err = d.consumeWhitespace(pos); err != nil {
				return pos, err
			}
		}
		switch c := d.buf[pos]; c {
		case ',':
			pos++
			continue
		case ']':
			pos++
			return pos, nil
		default:
			return pos, newInvalidCharacterError(c, "after array value (expecting ',' or ']')")
		}
	}
}

// InputOffset returns the current input byte offset. It gives the location
// of the next byte immediately after the most recently returned token or value.
// The number of bytes actually read from the underlying io.Reader may be more
// than this offset due to internal buffering effects.
func (d *Decoder) InputOffset() int64 {
	return d.previousOffsetEnd()
}

// UnreadBuffer returns the data remaining in the unread buffer,
// which may contain zero or more bytes.
// The returned buffer must not be mutated while Decoder continues to be used.
// The buffer contents are valid until the next Peek or Read call.
func (d *Decoder) UnreadBuffer() []byte {
	return d.unreadBuffer()
}

// StackDepth returns the depth of the state machine for read JSON data.
// Each level on the stack represents a nested JSON object or array.
// It is incremented whenever an ObjectStart or ArrayStart token is encountered
// and decremented whenever an ObjectEnd or ArrayEnd token is encountered.
// The depth is zero-indexed, where zero represents the top-level JSON value.
func (d *Decoder) StackDepth() int {
	// NOTE: Keep in sync with Encoder.StackDepth.
	return d.tokens.depth() - 1
}

// StackIndex returns information about the specified stack level.
// It must be a number between 0 and StackDepth, inclusive.
// For each level, it reports the kind:
//
//  • 0 for a level of zero,
//  • '{' for a level representing a JSON object, and
//  • '[' for a level representing a JSON array.
//
// and also reports the length of that JSON object or array.
// Each name and value in a JSON object is counted separately,
// so the effective number of members would be half the length.
// A complete JSON object must have an even length.
func (d *Decoder) StackIndex(i int) (Kind, int) {
	// NOTE: Keep in sync with Encoder.StackIndex.
	switch s := d.tokens[i]; {
	case i > 0 && s.isObject():
		return '{', s.length()
	case i > 0 && s.isArray():
		return '[', s.length()
	default:
		return 0, s.length()
	}
}

// consumeWhitespace consumes leading JSON whitespace per RFC 7159, section 2.
func consumeWhitespace(b []byte) (n int) {
	// NOTE: The arguments and logic are kept simple to keep this inlineable.
	for len(b) > n && (b[n] == ' ' || b[n] == '\t' || b[n] == '\r' || b[n] == '\n') {
		n++
	}
	return n
}

// consumeNull consumes the next JSON null literal per RFC 7159, section 3.
// It returns 0 if it is invalid, in which case consumeLiteral should be used.
func consumeNull(b []byte) int {
	// NOTE: The arguments and logic are kept simple to keep this inlineable.
	const literal = "null"
	if len(b) >= len(literal) && string(b[:len(literal)]) == literal {
		return len(literal)
	}
	return 0
}

// consumeFalse consumes the next JSON false literal per RFC 7159, section 3.
// It returns 0 if it is invalid, in which case consumeLiteral should be used.
func consumeFalse(b []byte) int {
	// NOTE: The arguments and logic are kept simple to keep this inlineable.
	const literal = "false"
	if len(b) >= len(literal) && string(b[:len(literal)]) == literal {
		return len(literal)
	}
	return 0
}

// consumeTrue consumes the next JSON true literal per RFC 7159, section 3.
// It returns 0 if it is invalid, in which case consumeLiteral should be used.
func consumeTrue(b []byte) int {
	// NOTE: The arguments and logic are kept simple to keep this inlineable.
	const literal = "true"
	if len(b) >= len(literal) && string(b[:len(literal)]) == literal {
		return len(literal)
	}
	return 0
}

// consumeLiteral consumes the next JSON literal per RFC 7159, section 3.
// If the input appears truncated, it returns io.ErrUnexpectedEOF.
func consumeLiteral(b []byte, lit string) (n int, err error) {
	for i := 0; i < len(b) && i < len(lit); i++ {
		if b[i] != lit[i] {
			return i, newInvalidCharacterError(b[i], "within literal "+lit+" (expecting "+escapeCharacter(lit[i])+")")
		}
	}
	if len(b) < len(lit) {
		return len(b), io.ErrUnexpectedEOF
	}
	return len(lit), nil
}

// consumeSimpleString consumes the next JSON string per RFC 7159, section 7
// but is limited to the grammar for an ASCII string without escape sequences.
// It returns 0 if it is invalid or more complicated than a simple string,
// in which case consumeString should be called.
func consumeSimpleString(b []byte) (n int) {
	// NOTE: The arguments and logic are kept simple to keep this inlineable.
	if len(b) > 0 && b[0] == '"' {
		n++
		for len(b) > n && (' ' <= b[n] && b[n] != '\\' && b[n] != '"' && b[n] <= unicode.MaxASCII) {
			n++
		}
		if len(b) > n && b[n] == '"' {
			n++
			return n
		}
	}
	return 0
}

// consumeString consumes the next JSON string per RFC 7159, section 7.
// If validateUTF8 is false, then this allows the presence of invalid UTF-8
// characters within the string itself.
// It reports the number of bytes consumed and whether an error was encounted.
// If the input appears truncated, it returns io.ErrUnexpectedEOF.
func consumeString(b []byte, validateUTF8 bool) (n int, err error) {
	return consumeStringResumable(b, 0, validateUTF8)
}

// consumeStringResumable is identical to consumeString but supports resuming
// from a previous call that returned io.ErrUnexpectedEOF.
func consumeStringResumable(b []byte, resumeOffset int, validateUTF8 bool) (n int, err error) {
	// Consume the leading quote.
	switch {
	case resumeOffset > 0:
		n = resumeOffset // already handled the leading quote
	case len(b) == 0:
		return n, io.ErrUnexpectedEOF
	case b[0] == '"':
		n++
	default:
		return n, newInvalidCharacterError(b[n], `at start of string (expecting '"')`)
	}

	// Consume every character in the string.
	for len(b) > n {
		// Optimize for long sequences of unescaped characters.
		for len(b) > n && (' ' <= b[n] && b[n] != '\\' && b[n] != '"' && b[n] <= unicode.MaxASCII) {
			n++
		}
		if len(b) == n {
			return n, io.ErrUnexpectedEOF
		}

		switch r, rn := utf8.DecodeRune(b[n:]); {
		case r == utf8.RuneError && rn == 1: // invalid UTF-8
			if validateUTF8 {
				if !utf8.FullRune(b[n:]) {
					return n, io.ErrUnexpectedEOF
				}
				return n, &SyntaxError{str: "invalid UTF-8 within string"}
			}
			n++
		case r < ' ': // invalid control character
			return n, newInvalidCharacterError(b[n], "within string (expecting non-control character)")
		case r == '"': // terminating quote
			n++
			return n, nil
		case r == '\\': // escape sequence
			resumeOffset = n
			if len(b) < n+2 {
				return resumeOffset, io.ErrUnexpectedEOF
			}
			switch r := b[n+1]; r {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				n += 2
			case 'u':
				if len(b) < n+6 {
					return resumeOffset, io.ErrUnexpectedEOF
				}
				v1, ok := parseHexUint16(b[n+2 : n+6])
				if !ok {
					return n, &SyntaxError{str: "invalid escape sequence " + strconv.Quote(string(b[n:n+6])) + " within string"}
				}
				n += 6

				if validateUTF8 && utf16.IsSurrogate(rune(v1)) {
					if len(b) >= n+2 && (b[n] != '\\' || b[n+1] != 'u') {
						return n, &SyntaxError{str: "invalid unpaired surrogate half within string"}
					}
					if len(b) < n+6 {
						return resumeOffset, io.ErrUnexpectedEOF
					}
					v2, ok := parseHexUint16(b[n+2 : n+6])
					if !ok {
						return n, &SyntaxError{str: "invalid escape sequence " + strconv.Quote(string(b[n:n+6])) + " within string"}
					}
					if utf16.DecodeRune(rune(v1), rune(v2)) == unicode.ReplacementChar {
						return n, &SyntaxError{str: "invalid surrogate pair in string"}
					}
					n += 6
				}
			default:
				return n, &SyntaxError{str: "invalid escape sequence " + strconv.Quote(string(b[n:n+2])) + " within string"}
			}
		default:
			n += rn
		}
	}
	return n, io.ErrUnexpectedEOF
}

// unescapeString append the unescaped for of a JSON string in src to dst.
// Any invalid UTF-8 within the string will be replaced with utf8.RuneError.
// The input must be an entire JSON string with no surrounding whitespace.
func unescapeString(dst, src []byte) (v []byte, ok bool) {
	// Consume quote delimiters.
	if len(src) < 1 || src[0] != '"' {
		return dst, false
	}
	src = src[1:]

	// Consume every character until completion.
	for len(src) > 0 {
		// Optimize for long sequences of unescaped characters.
		var n int
		for len(src) > n && (' ' <= src[n] && src[n] != '\\' && src[n] != '"' && src[n] <= unicode.MaxASCII) {
			n++
		}
		dst, src = append(dst, src[:n]...), src[n:]
		if len(src) == 0 {
			break
		}

		switch r, rn := utf8.DecodeRune(src); {
		case r == utf8.RuneError && rn == 1:
			// NOTE: An unescaped string may be longer than the escaped string
			// because invalid UTF-8 bytes are being replaced.
			dst, src = append(dst, "\uFFFD"...), src[1:]
		case r < ' ':
			return dst, false // invalid control character or unescaped quote
		case r == '"':
			src = src[1:]
			return dst, len(src) == 0
		case r == '\\':
			if len(src) < 2 {
				return dst, false // truncated escape sequence
			}
			switch r := src[1]; r {
			case '"', '\\', '/':
				dst, src = append(dst, r), src[2:]
			case 'b':
				dst, src = append(dst, '\b'), src[2:]
			case 'f':
				dst, src = append(dst, '\f'), src[2:]
			case 'n':
				dst, src = append(dst, '\n'), src[2:]
			case 'r':
				dst, src = append(dst, '\r'), src[2:]
			case 't':
				dst, src = append(dst, '\t'), src[2:]
			case 'u':
				if len(src) < 6 {
					return dst, false // truncated escape sequence
				}
				v1, ok := parseHexUint16(src[2:6])
				if !ok {
					return dst, false // invalid escape sequence
				}
				src = src[6:]

				// Check whether this is a surrogate halve.
				r := rune(v1)
				if utf16.IsSurrogate(r) {
					r = unicode.ReplacementChar // assume failure unless the following succeeds
					if len(src) >= 6 && src[0] == '\\' && src[1] == 'u' {
						if v2, ok := parseHexUint16(src[2:6]); ok {
							if r = utf16.DecodeRune(rune(v1), rune(v2)); r != unicode.ReplacementChar {
								src = src[6:]
							}
						}
					}
				}

				var arr [utf8.UTFMax]byte
				dst = append(dst, arr[:utf8.EncodeRune(arr[:], r)]...)
			default:
				return dst, false // invalid escape sequence
			}
		default:
			dst, src = append(dst, src[:rn]...), src[rn:]
		}
	}

	return dst, false // truncated input
}

// unescapeSimpleString returns the unescaped form of src.
// If there are no escaped characters, the output is simply a subslice of
// the input with the surrounding quotes removed.
// Otherwise, a new buffer is allocated for the output.
func unescapeSimpleString(src []byte) []byte {
	if consumeSimpleString(src) == len(src) {
		return src[len(`"`) : len(src)-len(`"`)]
	}
	out, ok := unescapeString(nil, src)
	if !ok {
		panic("BUG: invalid JSON string")
	}
	return out
}

// consumeSimpleNumber consumes the next JSON number per RFC 7159, section 6
// but is limited to the grammar for a positive integer.
// It returns 0 if it is invalid or more complicated than a simple integer,
// in which case consumeNumber should be called.
func consumeSimpleNumber(b []byte) (n int) {
	// NOTE: The arguments and logic are kept simple to keep this inlineable.
	if len(b) > 0 {
		if b[0] == '0' {
			n++
		} else if '1' <= b[0] && b[0] <= '9' {
			n++
			for len(b) > n && ('0' <= b[n] && b[n] <= '9') {
				n++
			}
		} else {
			return 0
		}
		if len(b) == n || !(b[n] == '.' || b[n] == 'e' || b[n] == 'E') {
			return n
		}
	}
	return 0
}

type consumeNumberState uint

const (
	consumeNumberInit consumeNumberState = iota
	beforeIntegerDigits
	withinIntegerDigits
	beforeFractionalDigits
	withinFractionalDigits
	beforeExponentDigits
	withinExponentDigits
)

// consumeNumber consumes the next JSON number per RFC 7159, section 6.
// It reports the number of bytes consumed and whether an error was encounted.
// If the input appears truncated, it returns io.ErrUnexpectedEOF.
//
// Note that JSON numbers are not self-terminating.
// If the entire input is consumed, then the caller needs to consider whether
// there may be subsequent unread data that may still be part of this number.
func consumeNumber(b []byte) (n int, err error) {
	n, _, err = consumeNumberResumable(b, 0, consumeNumberInit)
	return n, err
}

// consumeNumberResumable is identical to consumeNumber but supports resuming
// from a previous call that returned io.ErrUnexpectedEOF.
func consumeNumberResumable(b []byte, resumeOffset int, state consumeNumberState) (n int, _ consumeNumberState, err error) {
	// Jump to the right state when resuming from a partial consumption.
	n = resumeOffset
	if state > consumeNumberInit {
		switch state {
		case withinIntegerDigits, withinFractionalDigits, withinExponentDigits:
			// Consume leading digits.
			for len(b) > n && ('0' <= b[n] && b[n] <= '9') {
				n++
			}
			if len(b) == n {
				return n, state, nil // still within the same state
			}
			state++ // switches "withinX" to "beforeY" where Y is the state after X
		}
		switch state {
		case beforeIntegerDigits:
			goto beforeInteger
		case beforeFractionalDigits:
			goto beforeFractional
		case beforeExponentDigits:
			goto beforeExponent
		default:
			return n, state, nil
		}
	}

	// Consume required integer component (with optional minus sign).
beforeInteger:
	resumeOffset = n
	if len(b) > 0 && b[0] == '-' {
		n++
	}
	switch {
	case len(b) == n:
		return resumeOffset, beforeIntegerDigits, io.ErrUnexpectedEOF
	case b[n] == '0':
		n++
		state = beforeFractionalDigits
	case '1' <= b[n] && b[n] <= '9':
		n++
		for len(b) > n && ('0' <= b[n] && b[n] <= '9') {
			n++
		}
		state = withinIntegerDigits
	default:
		return n, state, newInvalidCharacterError(b[n], "within number (expecting digit)")
	}

	// Consume optional fractional component.
beforeFractional:
	if len(b) > n && b[n] == '.' {
		resumeOffset = n
		n++
		switch {
		case len(b) == n:
			return resumeOffset, beforeFractionalDigits, io.ErrUnexpectedEOF
		case '0' <= b[n] && b[n] <= '9':
			n++
		default:
			return n, state, newInvalidCharacterError(b[n], "within number (expecting digit)")
		}
		for len(b) > n && ('0' <= b[n] && b[n] <= '9') {
			n++
		}
		state = withinFractionalDigits
	}

	// Consume optional exponent component.
beforeExponent:
	if len(b) > n && (b[n] == 'e' || b[n] == 'E') {
		resumeOffset = n
		n++
		if len(b) > n && (b[n] == '-' || b[n] == '+') {
			n++
		}
		switch {
		case len(b) == n:
			return resumeOffset, beforeExponentDigits, io.ErrUnexpectedEOF
		case '0' <= b[n] && b[n] <= '9':
			n++
		default:
			return n, state, newInvalidCharacterError(b[n], "within number (expecting digit)")
		}
		for len(b) > n && ('0' <= b[n] && b[n] <= '9') {
			n++
		}
		state = withinExponentDigits
	}

	return n, state, nil
}

// parseHexUint16 is similar to strconv.ParseUint,
// but operates directly on []byte and is optimized for base-16.
// See https://golang.org/issue/42429.
func parseHexUint16(b []byte) (v uint16, ok bool) {
	if len(b) != 4 {
		return 0, false
	}
	for _, c := range b[:4] {
		switch {
		case '0' <= c && c <= '9':
			c = c - '0'
		case 'a' <= c && c <= 'f':
			c = 10 + c - 'a'
		case 'A' <= c && c <= 'F':
			c = 10 + c - 'A'
		default:
			return 0, false
		}
		v = v*16 + uint16(c)
	}
	return v, true
}

// parseDecUint is similar to strconv.ParseUint,
// but operates directly on []byte and is optimized for base-10.
// If the number is syntactically valid but overflows uint64,
// then it returns (math.MaxUint64, false).
// See https://golang.org/issue/42429.
func parseDecUint(b []byte) (v uint64, ok bool) {
	// Overflow logic is based on strconv/atoi.go:138-149 from Go1.15, where:
	//	• cutoff is equal to math.MaxUint64/10+1, and
	//	• the n1 > maxVal check is unnecessary
	//	  since maxVal is equivalent to math.MaxUint64.
	var n int
	var overflow bool
	for len(b) > n && ('0' <= b[n] && b[n] <= '9') {
		overflow = overflow || v >= math.MaxUint64/10+1
		v *= 10

		v1 := v + uint64(b[n]-'0')
		overflow = overflow || v1 < v
		v = v1

		n++
	}
	if n == 0 || len(b) != n {
		return 0, false
	}
	if overflow {
		return math.MaxUint64, false
	}
	return v, true
}

// parseFloat parses a floating point number according to the Go float grammar.
// Note that the JSON number grammar is a strict subset.
//
// If the number overflows the finite representation of a float,
// then we return MaxFloat since any finite value will always be infinitely
// more accurate at representing another finite value than an infinite value.
func parseFloat(b []byte, bits int) (v float64, ok bool) {
	// Note that the []byte->string conversion unfortunately allocates.
	// See https://golang.org/issue/42429 for more information.
	fv, err := strconv.ParseFloat(string(b), bits)
	if math.IsInf(fv, 0) {
		switch {
		case bits == 32 && math.IsInf(fv, +1):
			return +math.MaxFloat32, true
		case bits == 64 && math.IsInf(fv, +1):
			return +math.MaxFloat64, true
		case bits == 32 && math.IsInf(fv, -1):
			return -math.MaxFloat32, true
		case bits == 64 && math.IsInf(fv, -1):
			return -math.MaxFloat64, true
		}
	}
	return fv, err == nil
}
