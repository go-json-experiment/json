// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsontext

import (
	"bytes"
	"io"
	"math/bits"

	"github.com/go-json-experiment/json/internal/jsonflags"
	"github.com/go-json-experiment/json/internal/jsonopts"
	"github.com/go-json-experiment/json/internal/jsonwire"
)

// Encoder is a streaming encoder from raw JSON tokens and values.
// It is used to write a stream of top-level JSON values,
// each terminated with a newline character.
//
// [Encoder.WriteToken] and [Encoder.WriteValue] calls may be interleaved.
// For example, the following JSON value:
//
//	{"name":"value","array":[null,false,true,3.14159],"object":{"k":"v"}}
//
// can be composed with the following calls (ignoring errors for brevity):
//
//	e.WriteToken(ObjectStart)        // {
//	e.WriteToken(String("name"))     // "name"
//	e.WriteToken(String("value"))    // "value"
//	e.WriteValue(Value(`"array"`))   // "array"
//	e.WriteToken(ArrayStart)         // [
//	e.WriteToken(Null)               // null
//	e.WriteToken(False)              // false
//	e.WriteValue(Value("true"))      // true
//	e.WriteToken(Float(3.14159))     // 3.14159
//	e.WriteToken(ArrayEnd)           // ]
//	e.WriteValue(Value(`"object"`))  // "object"
//	e.WriteValue(Value(`{"k":"v"}`)) // {"k":"v"}
//	e.WriteToken(ObjectEnd)          // }
//
// The above is one of many possible sequence of calls and
// may not represent the most sensible method to call for any given token/value.
// For example, it is probably more common to call [Encoder.WriteToken] with a string
// for object names.
type Encoder struct {
	s encoderState
}

// encoderState is the low-level state of Encoder.
// It has exported fields and method for use by the "json" package.
type encoderState struct {
	state
	encodeBuffer
	jsonopts.Struct

	SeenPointers map[any]struct{} // only used when marshaling; identical to json.seenPointers
}

// encodeBuffer is a buffer split into 2 segments:
//
//   - buf[0:len(buf)]        // written (but unflushed) portion of the buffer
//   - buf[len(buf):cap(buf)] // unused portion of the buffer
type encodeBuffer struct {
	Buf []byte // may alias wr if it is a bytes.Buffer

	// baseOffset is added to len(buf) to obtain the absolute offset
	// relative to the start of io.Writer stream.
	baseOffset int64

	wr io.Writer

	// maxValue is the approximate maximum Value size passed to WriteValue.
	maxValue int
	// unusedCache is the buffer returned by the UnusedBuffer method.
	unusedCache []byte
	// bufStats is statistics about buffer utilization.
	// It is only used with pooled encoders in pools.go.
	bufStats bufferStatistics
}

// NewEncoder constructs a new streaming encoder writing to w
// configured with the provided options.
// It flushes the internal buffer when the buffer is sufficiently full or
// when a top-level value has been written.
//
// If w is a [bytes.Buffer], then the encoder appends directly into the buffer
// without copying the contents from an intermediate buffer.
func NewEncoder(w io.Writer, opts ...Options) *Encoder {
	e := new(Encoder)
	e.Reset(w, opts...)
	return e
}

// Reset resets an encoder such that it is writing afresh to w and
// configured with the provided options. Reset must not be called on
// a Encoder passed to the [encoding/json/v2.MarshalerV2.MarshalJSONV2] method
// or the [encoding/json/v2.MarshalFuncV2] function.
func (e *Encoder) Reset(w io.Writer, opts ...Options) {
	switch {
	case e == nil:
		panic("jsontext: invalid nil Encoder")
	case w == nil:
		panic("jsontext: invalid nil io.Writer")
	case e.s.Flags.Get(jsonflags.WithinArshalCall):
		panic("jsontext: cannot reset Encoder passed to json.MarshalerV2")
	}
	e.s.reset(nil, w, opts...)
}

func (e *encoderState) reset(b []byte, w io.Writer, opts ...Options) {
	e.state.reset()
	e.encodeBuffer = encodeBuffer{Buf: b, wr: w, bufStats: e.bufStats}
	if bb, ok := w.(*bytes.Buffer); ok && bb != nil {
		e.Buf = bb.Bytes()[bb.Len():] // alias the unused buffer of bb
	}
	e.Struct = jsonopts.Struct{}
	e.Struct.Join(opts...)
	if e.Flags.Get(jsonflags.Multiline) && !e.Flags.Has(jsonflags.Indent) {
		e.Indent = "\t"
	}
}

// NeedFlush determines whether to flush at this point.
func (e *encoderState) NeedFlush() bool {
	// NOTE: This function is carefully written to be inlinable.

	// Avoid flushing if e.wr is nil since there is no underlying writer.
	// Flush if less than 25% of the capacity remains.
	// Flushing at some constant fraction ensures that the buffer stops growing
	// so long as the largest Token or Value fits within that unused capacity.
	return e.wr != nil && (e.Tokens.Depth() == 1 || len(e.Buf) > 3*cap(e.Buf)/4)
}

// Flush flushes the buffer to the underlying io.Writer.
// It may append a trailing newline after the top-level value.
func (e *encoderState) Flush() error {
	if e.wr == nil || e.avoidFlush() {
		return nil
	}

	// In streaming mode, always emit a newline after the top-level value.
	if e.Tokens.Depth() == 1 && !e.Flags.Get(jsonflags.OmitTopLevelNewline) {
		e.Buf = append(e.Buf, '\n')
	}

	// Inform objectNameStack that we are about to flush the buffer content.
	e.Names.copyQuotedBuffer(e.Buf)

	// Specialize bytes.Buffer for better performance.
	if bb, ok := e.wr.(*bytes.Buffer); ok {
		// If e.buf already aliases the internal buffer of bb,
		// then the Write call simply increments the internal offset,
		// otherwise Write operates as expected.
		// See https://go.dev/issue/42986.
		n, _ := bb.Write(e.Buf) // never fails unless bb is nil
		e.baseOffset += int64(n)

		// If the internal buffer of bytes.Buffer is too small,
		// append operations elsewhere in the Encoder may grow the buffer.
		// This would be semantically correct, but hurts performance.
		// As such, ensure 25% of the current length is always available
		// to reduce the probability that other appends must allocate.
		if avail := bb.Available(); avail < bb.Len()/4 {
			bb.Grow(avail + 1)
		}

		e.Buf = bb.AvailableBuffer()
		return nil
	}

	// Flush the internal buffer to the underlying io.Writer.
	n, err := e.wr.Write(e.Buf)
	e.baseOffset += int64(n)
	if err != nil {
		// In the event of an error, preserve the unflushed portion.
		// Thus, write errors aren't fatal so long as the io.Writer
		// maintains consistent state after errors.
		if n > 0 {
			e.Buf = e.Buf[:copy(e.Buf, e.Buf[n:])]
		}
		return &ioError{action: "write", err: err}
	}
	e.Buf = e.Buf[:0]

	// Check whether to grow the buffer.
	// Note that cap(e.buf) may already exceed maxBufferSize since
	// an append elsewhere already grew it to store a large token.
	const maxBufferSize = 4 << 10
	const growthSizeFactor = 2 // higher value is faster
	const growthRateFactor = 2 // higher value is slower
	// By default, grow if below the maximum buffer size.
	grow := cap(e.Buf) <= maxBufferSize/growthSizeFactor
	// Growing can be expensive, so only grow
	// if a sufficient number of bytes have been processed.
	grow = grow && int64(cap(e.Buf)) < e.previousOffsetEnd()/growthRateFactor
	if grow {
		e.Buf = make([]byte, 0, cap(e.Buf)*growthSizeFactor)
	}

	return nil
}

// injectSyntacticErrorWithPosition wraps a SyntacticError with the position,
// otherwise it returns the error as is.
// It takes a position relative to the start of the start of e.buf.
func (e *encodeBuffer) injectSyntacticErrorWithPosition(err error, pos int) error {
	if serr, ok := err.(*SyntacticError); ok {
		return serr.withOffset(e.baseOffset + int64(pos))
	}
	return err
}

func (e *encodeBuffer) previousOffsetEnd() int64 { return e.baseOffset + int64(len(e.Buf)) }
func (e *encodeBuffer) unflushedBuffer() []byte  { return e.Buf }

// avoidFlush indicates whether to avoid flushing to ensure there is always
// enough in the buffer to unwrite the last object member if it were empty.
func (e *encoderState) avoidFlush() bool {
	switch {
	case e.Tokens.Last.Length() == 0:
		// Never flush after ObjectStart or ArrayStart since we don't know yet
		// if the object or array will end up being empty.
		return true
	case e.Tokens.Last.needObjectValue():
		// Never flush before the object value since we don't know yet
		// if the object value will end up being empty.
		return true
	case e.Tokens.Last.NeedObjectName() && len(e.Buf) >= 2:
		// Never flush after the object value if it does turn out to be empty.
		switch string(e.Buf[len(e.Buf)-2:]) {
		case `ll`, `""`, `{}`, `[]`: // last two bytes of every empty value
			return true
		}
	}
	return false
}

// UnwriteEmptyObjectMember unwrites the last object member if it is empty
// and reports whether it performed an unwrite operation.
func (e *encoderState) UnwriteEmptyObjectMember(prevName *string) bool {
	if last := e.Tokens.Last; !last.isObject() || !last.NeedObjectName() || last.Length() == 0 {
		panic("BUG: must be called on an object after writing a value")
	}

	// The flushing logic is modified to never flush a trailing empty value.
	// The encoder never writes trailing whitespace eagerly.
	b := e.unflushedBuffer()

	// Detect whether the last value was empty.
	var n int
	if len(b) >= 3 {
		switch string(b[len(b)-2:]) {
		case "ll": // last two bytes of `null`
			n = len(`null`)
		case `""`:
			// It is possible for a non-empty string to have `""` as a suffix
			// if the second to the last quote was escaped.
			if b[len(b)-3] == '\\' {
				return false // e.g., `"\""` is not empty
			}
			n = len(`""`)
		case `{}`:
			n = len(`{}`)
		case `[]`:
			n = len(`[]`)
		}
	}
	if n == 0 {
		return false
	}

	// Unwrite the value, whitespace, colon, name, whitespace, and comma.
	b = b[:len(b)-n]
	b = jsonwire.TrimSuffixWhitespace(b)
	b = jsonwire.TrimSuffixByte(b, ':')
	b = jsonwire.TrimSuffixWhitespace(b)
	b = jsonwire.TrimSuffixString(b)
	b = jsonwire.TrimSuffixWhitespace(b)
	b = jsonwire.TrimSuffixByte(b, ',')
	e.Buf = b // store back truncated unflushed buffer

	// Undo state changes.
	e.Tokens.Last.decrement() // for object member value
	e.Tokens.Last.decrement() // for object member name
	if !e.Flags.Get(jsonflags.AllowDuplicateNames) {
		if e.Tokens.Last.isActiveNamespace() {
			e.Namespaces.Last().removeLast()
		}
		e.Names.clearLast()
		if prevName != nil {
			e.Names.copyQuotedBuffer(e.Buf) // required by objectNameStack.replaceLastUnquotedName
			e.Names.replaceLastUnquotedName(*prevName)
		}
	}
	return true
}

// UnwriteOnlyObjectMemberName unwrites the only object member name
// and returns the unquoted name.
func (e *encoderState) UnwriteOnlyObjectMemberName() string {
	if last := e.Tokens.Last; !last.isObject() || last.Length() != 1 {
		panic("BUG: must be called on an object after writing first name")
	}

	// Unwrite the name and whitespace.
	b := jsonwire.TrimSuffixString(e.Buf)
	isVerbatim := bytes.IndexByte(e.Buf[len(b):], '\\') < 0
	name := string(jsonwire.UnquoteMayCopy(e.Buf[len(b):], isVerbatim))
	e.Buf = jsonwire.TrimSuffixWhitespace(b)

	// Undo state changes.
	e.Tokens.Last.decrement()
	if !e.Flags.Get(jsonflags.AllowDuplicateNames) {
		if e.Tokens.Last.isActiveNamespace() {
			e.Namespaces.Last().removeLast()
		}
		e.Names.clearLast()
	}
	return name
}

// WriteToken writes the next token and advances the internal write offset.
//
// The provided token kind must be consistent with the JSON grammar.
// For example, it is an error to provide a number when the encoder
// is expecting an object name (which is always a string), or
// to provide an end object delimiter when the encoder is finishing an array.
// If the provided token is invalid, then it reports a [SyntacticError] and
// the internal state remains unchanged. The offset reported
// in [SyntacticError] will be relative to the [Encoder.OutputOffset].
func (e *Encoder) WriteToken(t Token) error {
	return e.s.WriteToken(t)
}
func (e *encoderState) WriteToken(t Token) error {
	k := t.Kind()
	b := e.Buf // use local variable to avoid mutating e in case of error

	// Append any delimiters or optional whitespace.
	b = e.Tokens.MayAppendDelim(b, k)
	if e.Flags.Get(jsonflags.AnyWhitespace) {
		b = e.appendWhitespace(b, k)
	}
	pos := len(b) // offset before the token

	// Append the token to the output and to the state machine.
	var err error
	switch k {
	case 'n':
		b = append(b, "null"...)
		err = e.Tokens.appendLiteral()
	case 'f':
		b = append(b, "false"...)
		err = e.Tokens.appendLiteral()
	case 't':
		b = append(b, "true"...)
		err = e.Tokens.appendLiteral()
	case '"':
		if b, err = t.appendString(b, &e.Flags); err != nil {
			break
		}
		if !e.Flags.Get(jsonflags.AllowDuplicateNames) && e.Tokens.Last.NeedObjectName() {
			if !e.Tokens.Last.isValidNamespace() {
				err = errInvalidNamespace
				break
			}
			if e.Tokens.Last.isActiveNamespace() && !e.Namespaces.Last().insertQuoted(b[pos:], false) {
				err = newDuplicateNameError(b[pos:])
				break
			}
			e.Names.ReplaceLastQuotedOffset(pos) // only replace if insertQuoted succeeds
		}
		err = e.Tokens.appendString()
	case '0':
		if b, err = t.appendNumber(b, e.Flags.Get(jsonflags.CanonicalizeNumbers)); err != nil {
			break
		}
		err = e.Tokens.appendNumber()
	case '{':
		b = append(b, '{')
		if err = e.Tokens.pushObject(); err != nil {
			break
		}
		if !e.Flags.Get(jsonflags.AllowDuplicateNames) {
			e.Names.push()
			e.Namespaces.push()
		}
	case '}':
		b = append(b, '}')
		if err = e.Tokens.popObject(); err != nil {
			break
		}
		if !e.Flags.Get(jsonflags.AllowDuplicateNames) {
			e.Names.pop()
			e.Namespaces.pop()
		}
	case '[':
		b = append(b, '[')
		err = e.Tokens.pushArray()
	case ']':
		b = append(b, ']')
		err = e.Tokens.popArray()
	default:
		err = &SyntacticError{str: "invalid json.Token"}
	}
	if err != nil {
		return e.injectSyntacticErrorWithPosition(err, pos)
	}

	// Finish off the buffer and store it back into e.
	e.Buf = b
	if e.NeedFlush() {
		return e.Flush()
	}
	return nil
}

// AppendRaw appends either a raw string (without double quotes) or number.
// Specify safeASCII if the string output is guaranteed to be ASCII
// without any characters (including '<', '>', and '&') that need escaping,
// otherwise this will validate whether the string needs escaping.
// The appended bytes for a JSON number must be valid.
//
// This is a specialized implementation of Encoder.WriteValue
// that allows appending directly into the buffer.
// It is only called from marshal logic in the "json" package.
func (e *encoderState) AppendRaw(k Kind, safeASCII bool, appendFn func([]byte) ([]byte, error)) error {
	b := e.Buf // use local variable to avoid mutating e in case of error

	// Append any delimiters or optional whitespace.
	b = e.Tokens.MayAppendDelim(b, k)
	if e.Flags.Get(jsonflags.AnyWhitespace) {
		b = e.appendWhitespace(b, k)
	}
	pos := len(b) // offset before the token

	var err error
	switch k {
	case '"':
		// Append directly into the encoder buffer by assuming that
		// most of the time none of the characters need escaping.
		b = append(b, '"')
		if b, err = appendFn(b); err != nil {
			return err
		}
		b = append(b, '"')

		// Check whether we need to escape the string and if necessary
		// copy it to a scratch buffer and then escape it back.
		isVerbatim := safeASCII || !jsonwire.NeedEscape(b[pos+len(`"`):len(b)-len(`"`)])
		if !isVerbatim {
			var err error
			b2 := append(e.unusedCache, b[pos+len(`"`):len(b)-len(`"`)]...)
			b, err = jsonwire.AppendQuote(b[:pos], string(b2), &e.Flags)
			e.unusedCache = b2[:0]
			if err != nil {
				return e.injectSyntacticErrorWithPosition(err, pos)
			}
		}

		// Update the state machine.
		if !e.Flags.Get(jsonflags.AllowDuplicateNames) && e.Tokens.Last.NeedObjectName() {
			if !e.Tokens.Last.isValidNamespace() {
				return errInvalidNamespace
			}
			if e.Tokens.Last.isActiveNamespace() && !e.Namespaces.Last().insertQuoted(b[pos:], isVerbatim) {
				err := newDuplicateNameError(b[pos:])
				return e.injectSyntacticErrorWithPosition(err, pos)
			}
			e.Names.ReplaceLastQuotedOffset(pos) // only replace if insertQuoted succeeds
		}
		if err := e.Tokens.appendString(); err != nil {
			return e.injectSyntacticErrorWithPosition(err, pos)
		}
	case '0':
		if b, err = appendFn(b); err != nil {
			return err
		}
		if err := e.Tokens.appendNumber(); err != nil {
			return e.injectSyntacticErrorWithPosition(err, pos)
		}
	default:
		panic("BUG: invalid kind")
	}

	// Finish off the buffer and store it back into e.
	e.Buf = b
	if e.NeedFlush() {
		return e.Flush()
	}
	return nil
}

// WriteValue writes the next raw value and advances the internal write offset.
// The Encoder does not simply copy the provided value verbatim, but
// parses it to ensure that it is syntactically valid and reformats it
// according to how the Encoder is configured to format whitespace and strings.
// If [AllowInvalidUTF8] is specified, then any invalid UTF-8 is mangled
// as the Unicode replacement character, U+FFFD.
//
// The provided value kind must be consistent with the JSON grammar
// (see examples on [Encoder.WriteToken]). If the provided value is invalid,
// then it reports a [SyntacticError] and the internal state remains unchanged.
// The offset reported in [SyntacticError] will be relative to the
// [Encoder.OutputOffset] plus the offset into v of any encountered syntax error.
func (e *Encoder) WriteValue(v Value) error {
	return e.s.WriteValue(v)
}
func (e *encoderState) WriteValue(v Value) error {
	e.maxValue |= len(v) // bitwise OR is a fast approximation of max

	k := v.Kind()
	b := e.Buf // use local variable to avoid mutating e in case of error

	// Append any delimiters or optional whitespace.
	b = e.Tokens.MayAppendDelim(b, k)
	if e.Flags.Get(jsonflags.AnyWhitespace) {
		b = e.appendWhitespace(b, k)
	}
	pos := len(b) // offset before the value

	// Append the value the output.
	var n int
	n += jsonwire.ConsumeWhitespace(v[n:])
	b, m, err := e.reformatValue(b, v[n:], e.Tokens.Depth())
	if err != nil {
		return e.injectSyntacticErrorWithPosition(err, pos+n+m)
	}
	n += m
	n += jsonwire.ConsumeWhitespace(v[n:])
	if len(v) > n {
		err = newInvalidCharacterError(v[n:], "after top-level value")
		return e.injectSyntacticErrorWithPosition(err, pos+n)
	}

	// Append the kind to the state machine.
	switch k {
	case 'n', 'f', 't':
		err = e.Tokens.appendLiteral()
	case '"':
		if !e.Flags.Get(jsonflags.AllowDuplicateNames) && e.Tokens.Last.NeedObjectName() {
			if !e.Tokens.Last.isValidNamespace() {
				err = errInvalidNamespace
				break
			}
			if e.Tokens.Last.isActiveNamespace() && !e.Namespaces.Last().insertQuoted(b[pos:], false) {
				err = newDuplicateNameError(b[pos:])
				break
			}
			e.Names.ReplaceLastQuotedOffset(pos) // only replace if insertQuoted succeeds
		}
		err = e.Tokens.appendString()
	case '0':
		err = e.Tokens.appendNumber()
	case '{':
		if err = e.Tokens.pushObject(); err != nil {
			break
		}
		if err = e.Tokens.popObject(); err != nil {
			panic("BUG: popObject should never fail immediately after pushObject: " + err.Error())
		}
	case '[':
		if err = e.Tokens.pushArray(); err != nil {
			break
		}
		if err = e.Tokens.popArray(); err != nil {
			panic("BUG: popArray should never fail immediately after pushArray: " + err.Error())
		}
	}
	if err != nil {
		return e.injectSyntacticErrorWithPosition(err, pos)
	}

	// Finish off the buffer and store it back into e.
	e.Buf = b
	if e.NeedFlush() {
		return e.Flush()
	}
	return nil
}

// appendWhitespace appends whitespace that immediately precedes the next token.
func (e *encoderState) appendWhitespace(b []byte, next Kind) []byte {
	if delim := e.Tokens.needDelim(next); delim == ':' {
		if e.Flags.Get(jsonflags.Multiline | jsonflags.SpaceAfterColon) {
			return append(b, ' ')
		}
	} else {
		if e.Flags.Get(jsonflags.Multiline) {
			return e.AppendIndent(b, e.Tokens.NeedIndent(next))
		}
		if delim == ',' && e.Flags.Get(jsonflags.SpaceAfterComma) {
			return append(b, ' ')
		}
	}
	return b
}

// AppendIndent appends the appropriate number of indentation characters
// for the current nested level, n.
func (e *encoderState) AppendIndent(b []byte, n int) []byte {
	if n == 0 {
		return b
	}
	b = append(b, '\n')
	b = append(b, e.IndentPrefix...)
	for ; n > 1; n-- {
		b = append(b, e.Indent...)
	}
	return b
}

// reformatValue parses a JSON value from the start of src and
// appends it to the end of dst, reformatting whitespace and strings as needed.
// It returns the extended dst buffer and the number of consumed input bytes.
func (e *encoderState) reformatValue(dst []byte, src Value, depth int) ([]byte, int, error) {
	// TODO: Should this update ValueFlags as input?
	if len(src) == 0 {
		return dst, 0, io.ErrUnexpectedEOF
	}
	switch k := Kind(src[0]).normalize(); k {
	case 'n':
		if jsonwire.ConsumeNull(src) == 0 {
			n, err := jsonwire.ConsumeLiteral(src, "null")
			return dst, n, err
		}
		return append(dst, "null"...), len("null"), nil
	case 'f':
		if jsonwire.ConsumeFalse(src) == 0 {
			n, err := jsonwire.ConsumeLiteral(src, "false")
			return dst, n, err
		}
		return append(dst, "false"...), len("false"), nil
	case 't':
		if jsonwire.ConsumeTrue(src) == 0 {
			n, err := jsonwire.ConsumeLiteral(src, "true")
			return dst, n, err
		}
		return append(dst, "true"...), len("true"), nil
	case '"':
		if n := jsonwire.ConsumeSimpleString(src); n > 0 {
			dst, src = append(dst, src[:n]...), src[n:] // copy simple strings verbatim
			return dst, n, nil
		}
		return jsonwire.ReformatString(dst, src, &e.Flags)
	case '0':
		if n := jsonwire.ConsumeSimpleNumber(src); n > 0 && !e.Flags.Get(jsonflags.CanonicalizeNumbers) {
			dst, src = append(dst, src[:n]...), src[n:] // copy simple numbers verbatim
			return dst, n, nil
		}
		return jsonwire.ReformatNumber(dst, src, e.Flags.Get(jsonflags.CanonicalizeNumbers))
	case '{':
		return e.reformatObject(dst, src, depth)
	case '[':
		return e.reformatArray(dst, src, depth)
	default:
		return dst, 0, newInvalidCharacterError(src, "at start of value")
	}
}

// reformatObject parses a JSON object from the start of src and
// appends it to the end of src, reformatting whitespace and strings as needed.
// It returns the extended dst buffer and the number of consumed input bytes.
func (e *encoderState) reformatObject(dst []byte, src Value, depth int) ([]byte, int, error) {
	// Append object start.
	if len(src) == 0 || src[0] != '{' {
		panic("BUG: reformatObject must be called with a buffer that starts with '{'")
	} else if depth == maxNestingDepth+1 {
		return dst, 0, errMaxDepth
	}
	dst = append(dst, '{')
	n := len("{")

	// Append (possible) object end.
	n += jsonwire.ConsumeWhitespace(src[n:])
	if uint(len(src)) <= uint(n) {
		return dst, n, io.ErrUnexpectedEOF
	}
	if src[n] == '}' {
		dst = append(dst, '}')
		n += len("}")
		return dst, n, nil
	}

	var err error
	var names *objectNamespace
	if !e.Flags.Get(jsonflags.AllowDuplicateNames) {
		e.Namespaces.push()
		defer e.Namespaces.pop()
		names = e.Namespaces.Last()
	}
	depth++
	for {
		// Append optional newline and indentation.
		if e.Flags.Get(jsonflags.Multiline) {
			dst = e.AppendIndent(dst, depth)
		}

		// Append object name.
		n += jsonwire.ConsumeWhitespace(src[n:])
		if uint(len(src)) <= uint(n) {
			return dst, n, io.ErrUnexpectedEOF
		}
		m := jsonwire.ConsumeSimpleString(src[n:])
		if m > 0 {
			dst = append(dst, src[n:n+m]...)
		} else {
			dst, m, err = jsonwire.ReformatString(dst, src[n:], &e.Flags)
			if err != nil {
				return dst, n + m, err
			}
		}
		// TODO: Specify whether the name is verbatim or not.
		if !e.Flags.Get(jsonflags.AllowDuplicateNames) && !names.insertQuoted(src[n:n+m], false) {
			return dst, n, newDuplicateNameError(src[n : n+m])
		}
		n += m

		// Append colon.
		n += jsonwire.ConsumeWhitespace(src[n:])
		if uint(len(src)) <= uint(n) {
			return dst, n, io.ErrUnexpectedEOF
		}
		if src[n] != ':' {
			return dst, n, newInvalidCharacterError(src[n:], "after object name (expecting ':')")
		}
		dst = append(dst, ':')
		n += len(":")
		if e.Flags.Get(jsonflags.Multiline | jsonflags.SpaceAfterColon) {
			dst = append(dst, ' ')
		}

		// Append object value.
		n += jsonwire.ConsumeWhitespace(src[n:])
		if uint(len(src)) <= uint(n) {
			return dst, n, io.ErrUnexpectedEOF
		}
		dst, m, err = e.reformatValue(dst, src[n:], depth)
		if err != nil {
			return dst, n + m, err
		}
		n += m

		// Append comma or object end.
		n += jsonwire.ConsumeWhitespace(src[n:])
		if uint(len(src)) <= uint(n) {
			return dst, n, io.ErrUnexpectedEOF
		}
		switch src[n] {
		case ',':
			if e.Flags.Get(jsonflags.SpaceAfterComma) {
				dst = append(dst, ',', ' ')
			} else {
				dst = append(dst, ',')
			}
			n += len(",")
			continue
		case '}':
			if e.Flags.Get(jsonflags.Multiline) {
				dst = e.AppendIndent(dst, depth-1)
			}
			dst = append(dst, '}')
			n += len("}")
			return dst, n, nil
		default:
			return dst, n, newInvalidCharacterError(src[n:], "after object value (expecting ',' or '}')")
		}
	}
}

// reformatArray parses a JSON array from the start of src and
// appends it to the end of dst, reformatting whitespace and strings as needed.
// It returns the extended dst buffer and the number of consumed input bytes.
func (e *encoderState) reformatArray(dst []byte, src Value, depth int) ([]byte, int, error) {
	// Append array start.
	if len(src) == 0 || src[0] != '[' {
		panic("BUG: reformatArray must be called with a buffer that starts with '['")
	} else if depth == maxNestingDepth+1 {
		return dst, 0, errMaxDepth
	}
	dst = append(dst, '[')
	n := len("[")

	// Append (possible) array end.
	n += jsonwire.ConsumeWhitespace(src[n:])
	if uint(len(src)) <= uint(n) {
		return dst, n, io.ErrUnexpectedEOF
	}
	if src[n] == ']' {
		dst = append(dst, ']')
		n += len("]")
		return dst, n, nil
	}

	var err error
	depth++
	for {
		// Append optional newline and indentation.
		if e.Flags.Get(jsonflags.Multiline) {
			dst = e.AppendIndent(dst, depth)
		}

		// Append array value.
		n += jsonwire.ConsumeWhitespace(src[n:])
		if uint(len(src)) <= uint(n) {
			return dst, n, io.ErrUnexpectedEOF
		}
		var m int
		dst, m, err = e.reformatValue(dst, src[n:], depth)
		if err != nil {
			return dst, n + m, err
		}
		n += m

		// Append comma or array end.
		n += jsonwire.ConsumeWhitespace(src[n:])
		if uint(len(src)) <= uint(n) {
			return dst, n, io.ErrUnexpectedEOF
		}
		switch src[n] {
		case ',':
			if e.Flags.Get(jsonflags.SpaceAfterComma) {
				dst = append(dst, ',', ' ')
			} else {
				dst = append(dst, ',')
			}
			n += len(",")
			continue
		case ']':
			if e.Flags.Get(jsonflags.Multiline) {
				dst = e.AppendIndent(dst, depth-1)
			}
			dst = append(dst, ']')
			n += len("]")
			return dst, n, nil
		default:
			return dst, n, newInvalidCharacterError(src[n:], "after array value (expecting ',' or ']')")
		}
	}
}

// OutputOffset returns the current output byte offset. It gives the location
// of the next byte immediately after the most recently written token or value.
// The number of bytes actually written to the underlying [io.Writer] may be less
// than this offset due to internal buffering effects.
func (e *Encoder) OutputOffset() int64 {
	return e.s.previousOffsetEnd()
}

// UnusedBuffer returns a zero-length buffer with a possible non-zero capacity.
// This buffer is intended to be used to populate a [Value]
// being passed to an immediately succeeding [Encoder.WriteValue] call.
//
// Example usage:
//
//	b := d.UnusedBuffer()
//	b = append(b, '"')
//	b = appendString(b, v) // append the string formatting of v
//	b = append(b, '"')
//	... := d.WriteValue(b)
//
// It is the user's responsibility to ensure that the value is valid JSON.
func (e *Encoder) UnusedBuffer() []byte {
	// NOTE: We don't return e.buf[len(e.buf):cap(e.buf)] since WriteValue would
	// need to take special care to avoid mangling the data while reformatting.
	// WriteValue can't easily identify whether the input Value aliases e.buf
	// without using unsafe.Pointer. Thus, we just return a different buffer.
	// Should this ever alias e.buf, we need to consider how it operates with
	// the specialized performance optimization for bytes.Buffer.
	n := 1 << bits.Len(uint(e.s.maxValue|63)) // fast approximation for max length
	if cap(e.s.unusedCache) < n {
		e.s.unusedCache = make([]byte, 0, n)
	}
	return e.s.unusedCache
}

// StackDepth returns the depth of the state machine for written JSON data.
// Each level on the stack represents a nested JSON object or array.
// It is incremented whenever an [ObjectStart] or [ArrayStart] token is encountered
// and decremented whenever an [ObjectEnd] or [ArrayEnd] token is encountered.
// The depth is zero-indexed, where zero represents the top-level JSON value.
func (e *Encoder) StackDepth() int {
	// NOTE: Keep in sync with Decoder.StackDepth.
	return e.s.Tokens.Depth() - 1
}

// StackIndex returns information about the specified stack level.
// It must be a number between 0 and [Encoder.StackDepth], inclusive.
// For each level, it reports the kind:
//
//   - 0 for a level of zero,
//   - '{' for a level representing a JSON object, and
//   - '[' for a level representing a JSON array.
//
// It also reports the length of that JSON object or array.
// Each name and value in a JSON object is counted separately,
// so the effective number of members would be half the length.
// A complete JSON object must have an even length.
func (e *Encoder) StackIndex(i int) (Kind, int64) {
	// NOTE: Keep in sync with Decoder.StackIndex.
	switch s := e.s.Tokens.index(i); {
	case i > 0 && s.isObject():
		return '{', s.Length()
	case i > 0 && s.isArray():
		return '[', s.Length()
	default:
		return 0, s.Length()
	}
}

// StackPointer returns a JSON Pointer (RFC 6901) to the most recently written value.
// Object names are only present if [AllowDuplicateNames] is false, otherwise
// object members are represented using their index within the object.
func (e *Encoder) StackPointer() string {
	e.s.Names.copyQuotedBuffer(e.s.Buf)
	return string(e.s.appendStackPointer(nil))
}
