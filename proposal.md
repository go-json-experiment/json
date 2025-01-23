This is a discussion intended to lead to a formal proposal.

This was written with input from [@mvdan](https://github.com/mvdan), [@johanbrandhorst](https://github.com/johanbrandhorst), [@rogpeppe](https://github.com/rogpeppe), [@chrishines](https://github.com/chrishines), [@rsc](https://github.com/rsc).

# Background

The widely-used "encoding/json" package is over a decade old and has served the Go community well. Over time, we have learned much about what works well and what does not. Its ability to marshal from and unmarshal into native Go types, the ability to customize the representation of struct fields using Go struct tags, and the ability of Go types to customize their own representation has proven to be highly flexible.

However, [many flaws and shortcomings](https://github.com/golang/go/issues?q=is%3Aissue+is%3Aopen+%22encoding%2Fjson%22+in%3Atitle+sort%3Areactions-%2B1-desc) have also been identified over time. Addressing each issue in isolation will likely lead to surprising behavior when non-orthogonal features interact poorly. This discussion aims to take a cohesive and comprehensive look at "json" and to propose a solution to support JSON in Go for the next decade and beyond.

Improvements may be delivered either by adding new functionality to the existing "json" package and/or by introducing a new major version of the package. To guide this decision, let us evaluate the existing "json" package in the following categories:

1. Missing functionality
2. API deficiencies
3. Performance limitations
4. Behavioral flaws

## Missing functionality

There are [quite a number of open proposals](https://github.com/golang/go/issues?q=is%3Aissue+is%3Aopen+%22proposal%3A+encoding%2Fjson%22+in%3Atitle+sort%3Areactions-%2B1-desc+), where the most prominent feature requests are:
- The ability to specify custom formatting of `time.Time` ([#21990](https://github.com/golang/go/issues/21990))
- The ability to omit specific Go values when marshaling ([#11939](https://github.com/golang/go/issues/11939), [#22480](https://github.com/golang/go/issues/22480), [#50480](https://github.com/golang/go/issues/50480), [#29310](https://github.com/golang/go/issues/29310), [#52803](https://github.com/golang/go/issues/52803), [#45669](https://github.com/golang/go/issues/45669))
- The ability to marshal nil Go slices and maps as empty JSON arrays and objects ([#37711](https://github.com/golang/go/issues/37711), [#27589](https://github.com/golang/go/issues/27589))
- The ability to inline Go types without using Go embedding ([#6213](https://github.com/golang/go/issues/6213))

Most feature requests could be added to the existing "json" package in a backwards compatible way.

## API deficiencies

The API can be sharp or restrictive:

1. There is no easy way to correctly unmarshal from an `io.Reader`. Users often write `json.NewDecoder(r).Decode(v)`, which is incorrect since it does not reject trailing junk at the end of the payload ([#36225](https://github.com/golang/go/issues/36225)).

2. Options can be set on the `Encoder` and `Decoder` types, but cannot be used with the `Marshal` and `Unmarshal` functions. Similarly, types implementing the `Marshaler` and `Unmarshaler` interfaces cannot make use of the options. There is no way to plumb options down the call stack ([#41144](https://github.com/golang/go/issues/41144)).

3. The functions `Compact`, `Indent`, and `HTMLEscape` write to a `bytes.Buffer` instead of something more flexible like a `[]byte` or `io.Writer`. This limits the usability of those functions.

These deficiencies could be fixed by introducing new API to the existing "json" package in a backwards compatible way at the cost of introducing multiple different ways of accomplishing the same tasks in the same package.

## Performance limitations

The performance of the standard "json" package leaves much to be desired. Setting aside internal implementation details, there are externally visible APIs and behaviors that fundamentally limit performance:

1. **MarshalJSON**: The `MarshalJSON` interface method forces the implementation to allocate the returned `[]byte`. Also, the semantics require that the "json" package parse the result both to verify that it is valid JSON and also to reformat it to match the specified indentation.

2. **UnmarshalJSON**: The `UnmarshalJSON` interface method requires that a complete JSON value be provided (without any trailing data). This forces the "json" package to parse the JSON value to be unmarshaled in its entirety to determine when it ends before it can call `UnmarshalJSON`. Afterwards, the `UnmarshalJSON` method itself must parse the provided JSON value again. If the `UnmarshalJSON` implementation recursively calls `Unmarshal`, this leads to quadratic behavior. As an example, this is the source of dramatic performance degradation when unmarshaling into `spec.Swagger` ([kubernetes/kube-openapi#315](https://github.com/kubernetes/kube-openapi/issues/315)).

3. **Encoder.WriteToken**: There is no streaming encoder API. A proposal has been accepted but not implemented ([#40127](https://github.com/golang/go/issues/40127)). The proposed API symmetrically matches `Decoder.Token`, but suffers from the same performance problems (see next point).

4. **Decoder.Token**: The `Token` type is an interface, which can hold one of multiple types: `Delim`, `bool`, `float64`, `Number`, `string`, or `nil`. This unfortunately allocates frequently when boxing a number or string within the `Token` interface type ([#40128](https://github.com/golang/go/issues/40128)).

5. **Lack of streaming**: Even though the `Encoder.Encode` and `Decoder.Decode` methods operate on an `io.Writer` or `io.Reader`, they buffer the entire JSON value in memory. This hurts performance since it requires a second pass through the JSON. In theory, only the largest JSON token (i.e., a JSON string or number) should ever need to be buffered ([#33714](https://github.com/golang/go/issues/33714), [#7872](https://github.com/golang/go/issues/7872), [#11046](https://github.com/golang/go/issues/11046)).

Limitations 1 and 2 can be resolved by defining new interface methods that operate on a streaming encoder or decoder.

However, type-defined streaming methods are blocked on limitation 3 and 4, which requires having an efficient, streaming encoder and decoder API ([#40127](https://github.com/golang/go/issues/40127), [#40128](https://github.com/golang/go/issues/40128)).

Even if an efficient streaming API is provided, the "json" package itself would still be constrained by limitation 5, where it does not operate in a truly streaming manner under the hood ([#33714](https://github.com/golang/go/issues/33714), [#7872](https://github.com/golang/go/issues/7872), [#11046](https://github.com/golang/go/issues/11046)).

The "json" package should operate in a truly streaming manner by default when writing to or reading from an `io.Writer` or `io.Reader`. Buffering the entire JSON value defeats the point of using an `io.Reader` or `io.Writer`. Use cases that want to avoid outputting JSON in the event of an error should call `Marshal` instead and only write the output if the error is nil. Unfortunately, the "json" package cannot switch to streaming by default since this would be a breaking behavioral change, suggesting that a v2 "json" package is needed to accomplish this goal.

## Behavioral flaws

Various behavioral flaws have been identified with the "json" package:

1. **Improper handling of JSON syntax:** Over the years, JSON has seen increased amounts of standardization ([RFC 4627](https://tools.ietf.org/html/rfc4627), [RFC 7159](https://tools.ietf.org/html/rfc7159), [RFC 7493](https://tools.ietf.org/html/rfc7493), and [RFC 8259](https://tools.ietf.org/html/rfc8259)) in order for JSON-based protocols to properly communicate. Generally speaking, the specifications have gotten more strict over time since loose guarantees lead to implementations disagreeing about the meaning of a particular JSON value.

    - The "json" package currently allows invalid UTF-8, while the latest internet standard ([RFC 8259](https://tools.ietf.org/html/rfc8259)) requires valid UTF-8. The default behavior should at least be compliant with [RFC 8259](https://tools.ietf.org/html/rfc8259), which would require that the presence of invalid UTF-8 to be rejected as an error.

    - The "json" package currently allows for duplicate object member names. [RFC 8259](https://tools.ietf.org/html/rfc8259) specifies that duplicate object names result in unspecified behavior (e.g., an implementation may take the first value, last value, ignore it, reject it, etc.). Fundamentally, the presence of a duplicate object name results in a JSON value without any universally agreed upon semantic ([#43664](https://github.com/golang/go/issues/43664)). This could be [exploited by attackers in security applications](https://labs.bishopfox.com/tech-blog/an-exploration-of-json-interoperability-vulnerabilities) and has been [exploited in practice with severe consequences](https://www.cvedetails.com/cve/CVE-2017-12635/). The default behavior should err on the side of safety and reject duplicate names as recommended by [RFC 7493](https://tools.ietf.org/html/rfc7493).

    While the default behavior should be more strict, we should also provide an option for backwards compatibility to opt-in to the prior behavior of allowing invalid UTF-8 and/or allowing duplicate names.

2. **Case-insensitive unmarshaling:** When unmarshaling, JSON object names are paired with Go struct field names using a case-insensitive match ([#14750](https://github.com/golang/go/issues/14750)). This is a surprising default, a potential security vulnerability, and a performance limitation. It may be a security vulnerability when an attacker provides an [alternate encoding](https://capec.mitre.org/data/definitions/267.html) that a security tool does not know to check for. It is also a performance limitation since matching upon a case-insensitive name cannot be performed using a trivial Go map lookup.

3. **Inconsistent calling of type-defined methods:** Due to "json" and its use of Go reflection, the `MarshalJSON` and `UnmarshalJSON` methods cannot be called if the underlying value is not addressable ([#22967](https://github.com/golang/go/issues/22967), [#27722](https://github.com/golang/go/issues/27722), [#33993](https://github.com/golang/go/issues/33993), [#55890](https://github.com/golang/go/issues/55890)). This is surprising to users when their declared `MarshalJSON` and `UnmarshalJSON` methods are not called when the underlying Go value was retrieved through a Go map, interface, or another non-addressable value. The "json" package should consistently and always call the user-defined methods regardless of addressability. As an implementation detail, non-addressable values can always be made addressable by temporarily boxing them on the heap. This could arguably be considered a bug and be fixed in the current "json" package. However, previous attempts at fixing this resulted in the changes being reverted because it broke too many targets implicitly depending on the inconsistent calling behavior.

4. **Inconsistent merge semantics:** When unmarshaling into a non-empty Go value, the behavior is inconsistent about whether it clears the target, resets but reuses the target memory, and/or whether it merges into the target ([#27172](https://github.com/golang/go/issues/27172), [#31924](https://github.com/golang/go/issues/31924), [#26946](https://github.com/golang/go/issues/26946)). Most oddly, when unmarshaling into a non-nil Go slice, the unused elements between the length and capacity are merged into without being zeroed first ([#21092](https://github.com/golang/go/issues/21092)). The merge semantics of "json" came about organically without much thought given to a systematic approach to merging, leading to fragmented and inconsistent behavior.

5. **Inconsistent error values:** There are three classes of errors that can occur when handling JSON:

    - **Syntactic error:** The input does not match the JSON grammar (e.g., an improperly escaped JSON string).
    - **Semantic error:** The input is valid JSON, but there is a type-mismatch between the JSON value and the Go value (e.g., unmarshaling a JSON bool into a Go struct).
    - **I/O error:** A failure occurred writing to or reading from an `io.Writer` or `io.Reader`. This class of errors never occurs when marshaling to or unmarshaling from a `[]byte`.

    The "json" package is currently inconsistent about whether it returns structured or unstructured errors. It is currently impossible to reliably detect each class of error.

These behavioral flaws of "json" cannot be changed without being a breaking change. Options could be added to specify different behavior, but that would be unfortunate since the desired behavior is not the default behavior. Changing the default behavior suggests the need for a v2 "json" package.

# Proposal

The analysis above suggests that a new major version of the "json" package is necessary and worthwhile. In this section, we propose a rough draft of what a new major version could look like. Henceforth, we will refer to the existing "encoding/json" package as v1, and a hypothetical new major version as v2. This is a draft proposal as the proposed API and behavior is subject to change based on community discussion.

## Goals

Let us define some goals for v2:

- **Mostly backwards compatible:** If possible, v2 should aim to be mostly compatible with v1 in terms of both API and default behavior to ease migration. For example, the `Marshal` and `Unmarshal` functions are the most widely used declarations in v1. It is sensible for equivalent functionality in v2 to be named the same and have mostly the same signature. Behaviorally, we should aim for 95% to 99% backwards compatibility. We do not aim for 100% compatibility since we want the freedom to break certain behaviors that are now considered to have been a mistake.

- **More correct:** JSON standardization has become increasingly more strict over time due to interoperability issues. The default serialization should prioritize correctness.

- **More performant:** JSON serialization is widely used and performance gains translate to real-world resource savings. However, performance is secondary to correctness. For example, rejecting duplicate object names will hurt performance, but is the more correct behavior to have.

- **More flexible:** We should aim to provide the most flexible features that address most usages. We do not want to overfit v2 to handle every possible use case. The provided features should be orthogonal in nature such that any combination of features results in as few surprising edge cases as possible.

- **Easy to use (hard to misuse):** The API should aim to make the common case easy and the less common case at least possible. The API should avoid behavior that goes contrary to user expectation, which may result in subtle bugs.

- **Avoid unsafe:** JSON serialization is used by many internet-facing Go services. It is paramount that untrusted JSON inputs cannot result in memory corruption. Consequently, standard library packages generally avoid the use of package "unsafe" even if it could provide a performance boost. We aim to preserve this property.

    - There are many community forks or reimplementations of v1 "json". While they provide impressive performance gains, they cannot be adopted into the standard library on the basis of their extensive use of package "unsafe". The [2021 Go Developer Survey](https://go.dev/blog/survey2021-results#prioritization) shows that the assurance of reliability and security is a higher priority than CPU or memory performance.

## Overview

JSON serialization can be broken down into two primary components:

- **syntactic functionality** that is concerned with processing JSON based on its grammar, and
- **semantic functionality** that determines the meaning of JSON values as Go values and vice-versa.

We use the terms "encode" and "decode" to describe syntactic functionality and the terms "marshal" and "unmarshal" to describe semantic functionality. 

We aim to provide a clear distinction between functionality that is purely concerned with encoding versus that of marshaling. For example, it should be possible to  encode a stream of JSON tokens without needing to marshal a concrete Go value representing them. Similarly, it should be possible to  decode a stream of JSON tokens without needing to unmarshal them into a concrete Go value.

In v2, we propose that there be two packages: "jsontext" and "json". The "jsontext" package is concerned with syntactic functionality, while the "json" package is concerned with semantic functionality. The "json" package will be implemented in terms of the "jsontext" package. In order for "json" to marshal from and unmarshal into arbitrary Go values, it must have a dependency on the "reflect" package. In contrast, the "jsontext" package will have a relatively light dependency tree and be suitable for applications (e.g., [TinyGo](https://tinygo.org/), [GopherJS](https://github.com/gopherjs/gopherjs), [WASI](https://go.dev/blog/wasi), etc.) where binary bloat is a concern.

![block-diagram](https://raw.githubusercontent.com/go-json-experiment/json/6e475c84a2bf3c304682aef375e000771a318a5c/api.png)

This diagram provides a high-level overview of the v2 API. Purple blocks represent types, while blue blocks represent functions or methods. The direction of the arrows represent the approximate flow of data. The bottom half (as implemented by the "jsontext" package) of the diagram contains functionality that is only concerned with syntax, while the upper half (as implemented by the "json" package) contains functionality that assigns semantic meaning to syntactic data handled by the bottom half.

## The "jsontext" package

The `jsontext` package provides functionality to process JSON purely according to the grammar. This package will have a small dependency tree such that it results in minimal binary bloat. Most notably, it does not depend on Go reflection.

### Overview

The basic API consists of the following:
```go
package jsontext // "encoding/json/jsontext"

type Encoder struct { /* no exported fields */ }
func NewEncoder(io.Writer, ...Options) *Encoder
func (*Encoder) WriteToken(Token) error
func (*Encoder) WriteValue(Value) error

type Decoder struct { /* no exported fields */ }
func NewDecoder(io.Reader, ...Options) *Decoder
func (*Decoder) PeekKind() Kind
func (*Decoder) ReadToken() (Token, error)
func (*Decoder) ReadValue() (Value, error)
func (*Decoder) SkipValue() error

type Kind byte
type Token struct { /* no exported fields */ }
func (Token) Kind() Kind
type Value []byte
func (Value) Kind() Kind
```

### Values and Tokens

The primary data types for interacting with JSON are `Kind`, `Value`, and `Token`.

The `Kind` is an enumeration that describes the kind of a value or token.
```go
// Kind represents each possible JSON token kind with a single byte,
// which is the first byte of that kind's grammar:
//   - 'n': null
//   - 'f': false
//   - 't': true
//   - '"': string
//   - '0': number
//   - '{': object start
//   - '}': object end
//   - '[': array start
//   - ']': array end
type Kind byte
func (k Kind) String() string
```

A `Value` is the raw representation of a single JSON value, which can represent entire array or object values. It is analogous to the v1 `RawMessage` type.
```go
type Value []byte

func (v Value) Clone() Value
func (v Value) String() string
func (v Value) IsValid() bool
func (v *Value) Compact() error
func (v *Value) Indent(prefix, indent string) error
func (v *Value) Canonicalize() error
func (v Value) MarshalJSON() ([]byte, error)
func (v *Value) UnmarshalJSON(b []byte) error
func (v Value) Kind() Kind // never ']' or '}' if valid
```
The `Compact` and `Indent` methods operate similar to the v1 `Compact` and `Indent` function.
The `Canonicalize` method canonicalizes the JSON value according to the JSON Canonicalization Scheme as defined in [RFC 8785](https://www.rfc-editor.org/rfc/rfc8785).

A `Token` represents a lexical JSON token, which cannot represent entire array or object values. It is analogous to the v1 `Token` type, but is designed to be allocation-free by being an opaque struct type.
```go
type Token struct { /* no exported fields */ }

var (
	Null  Token = rawToken("null")
	False Token = rawToken("false")
	True  Token = rawToken("true")

	ObjectStart Token = rawToken("{")
	ObjectEnd   Token = rawToken("}")
	ArrayStart  Token = rawToken("[")
	ArrayEnd    Token = rawToken("]")
)

func Bool(b bool) Token
func Int(n int64) Token
func Uint(n uint64) Token
func Float(n float64) Token
func String(s string) Token
func (t Token) Clone() Token
func (t Token) Bool() bool
func (t Token) Int() int64
func (t Token) Uint() uint64
func (t Token) Float() float64
func (t Token) String() string
func (t Token) Kind() Kind
```

### Encoder and Decoder

The `Encoder` and `Decoder` types provide the functionality for encoding to or decoding from an `io.Writer` or an `io.Reader`. An `Encoder` or `Decoder` can be constructed with`NewEncoder` or `NewDecoder` using default options.

The `Encoder` is a streaming encoder from raw JSON tokens and values. It is used to write a stream of top-level JSON values, each terminated with a newline character.
```go
type Encoder struct { /* no exported fields */ }

func (e *Encoder) Reset(w io.Writer, opts ...Options)

// WriteToken writes the next token and advances the internal write offset.
// The provided token must be consistent with the JSON grammar.
func (e *Encoder) WriteToken(t Token) error

// WriteValue writes the next raw value and advances the internal write offset.
// The provided value must be consistent with the JSON grammar.
func (e *Encoder) WriteValue(v Value) error

// UnusedBuffer returns a zero-length buffer with a possible non-zero capacity.
// This buffer is intended to be used to populate a Value
// being passed to an immediately succeeding WriteValue call.
//
// Example usage:
//
//	b := d.UnusedBuffer()
//	b = append(b, '"')
//	b = appendString(b, v) // append the string formatting of v
//	b = append(b, '"')
//	... := d.WriteValue(b)
func (e *Encoder) UnusedBuffer() []byte

// OutputOffset returns the current output byte offset, which is the location
// of the next byte immediately after the most recently written token or value.
func (e *Encoder) OutputOffset() int64
```

The `Decoder` is a streaming decoder for raw JSON tokens and values. It is used to read a stream of top-level JSON values, each separated by optional whitespace characters.
```go
type Decoder struct { /* no exported fields */ }

func (d *Decoder) Reset(r io.Reader, opts ...Options)

// PeekKind returns the kind of the token that would be returned by ReadToken.
// It does not advance the read offset.
func (d *Decoder) PeekKind() Kind

// ReadToken reads the next Token, advancing the read offset.
// The returned token is only valid until the next Peek, Read, or Skip call.
// It returns io.EOF if there are no more tokens.
func (d *Decoder) ReadToken() (Token, error)

// ReadValue returns the next raw JSON value, advancing the read offset.
// The returned value is only valid until the next Peek, Read, or Skip call
// and may not be mutated while the Decoder remains in use.
// It returns io.EOF if there are no more values.
func (d *Decoder) ReadValue() (Value, error)

// SkipValue is equivalent to calling ReadValue and discarding the result except
// that memory is not wasted trying to hold the entire value.
func (d *Decoder) SkipValue() error

// UnreadBuffer returns the data remaining in the unread buffer.
// The returned buffer must not be mutated while Decoder continues to be used.
// The buffer contents are valid until the next Peek, Read, or Skip call.
func (d *Decoder) UnreadBuffer() []byte

// InputOffset returns the current input byte offset, which is the location
// of the next byte immediately after the most recently returned token or value.
func (d *Decoder) InputOffset() int64
```

Some methods common to both `Encoder` and `Decoder` report information about the current automaton state.
```go
// StackDepth returns the depth of the state machine.
// Each level on the stack represents a nested JSON object or array.
// It is incremented whenever an ObjectStart or ArrayStart token is encountered
// and decremented whenever an ObjectEnd or ArrayEnd token is encountered.
// The depth is zero-indexed, where zero represents the top-level JSON value.
func (e *Encoder) StackDepth() int
func (d *Decoder) StackDepth() int

// StackIndex returns information about the specified stack level.
// It must be a number between 0 and StackDepth, inclusive.
// For each level, it reports the kind:
//
//   - 0 for a level of zero,
//   - '{' for a level representing a JSON object, and
//   - '[' for a level representing a JSON array.
//
// It also reports the length so far of that JSON object or array.
// Each name and value in a JSON object is counted separately,
// so the effective number of members would be half the length.
// A complete JSON object must have an even length.
func (e *Encoder) StackIndex(i int) (Kind, int)
func (d *Decoder) StackIndex(i int) (Kind, int)

// StackPointer returns a JSON Pointer (RFC 6901) to the most recently handled value.
// Object names are only present if AllowDuplicateNames is false, otherwise
// object members are represented using their index within the object.
func (e *Encoder) StackPointer() string
func (d *Decoder) StackPointer() string
```

### Options

The behavior of `Encoder` and `Decoder` may be altered by passing options to `NewEncoder` and `NewDecoder`, which take in a variadic list of options. 
```go
type Options = jsonopts.Options

// AllowDuplicateNames specifies that JSON objects may contain
// duplicate member names.
func AllowDuplicateNames(v bool) Options // affects encode and decode

// AllowInvalidUTF8 specifies that JSON strings may contain invalid UTF-8,
// which will be mangled as the Unicode replacement character, U+FFFD.
func AllowInvalidUTF8(v bool) Options // affects encode and decode

// EscapeForHTML specifies that '<', '>', and '&' characters within JSON strings
// should be escaped as a hexadecimal Unicode codepoint (e.g., \u003c)
// so that the output is safe to embed within HTML.
func EscapeForHTML(v bool) Options // affects encode only

// EscapeForJS specifies that U+2028 and U+2029 characters within JSON strings
// should be escaped as a hexadecimal Unicode codepoint (e.g., \u2028)
// so that the output is valid to embed within JavaScript.
// See RFC 8259, section 12.
func EscapeForJS(v bool) Options // affects encode only

// WithIndent specifies that the encoder should emit multiline output
// where each element in a JSON object or array begins on a new, indented line
// beginning with the indent prefix (see WithIndentPrefix) followed by
// one or more copies of indent according to the nesting depth.
func WithIndent(indent string) Options // affects encode only

// WithIndentPrefix specifies that the encoder should emit multiline output
// where each element in a JSON object or array begins on a new, indented line
// beginning with the indent prefix followed by
// one or more copies of indent (see WithIndent) according to the nesting depth.
func WithIndentPrefix(prefix string) Options // affects encode only

// Expand specifies that the JSON output should be expanded, where
// every JSON object member or JSON array element appears on a new, indented line
// according to the nesting depth.
// If an indent is not already specified, then it defaults to using "\t".
func Expand(v bool) Options // affects encode only
```
The `Options` type is a type alias to an internal type that is an interface type with no exported methods. It is used simply as a marker type for options declared in the "json" and "jsontext" package.

Latter option specified in the variadic list passed to `NewEncoder` and `NewDecoder` takes precedence over prior option values. For example, `NewEncoder(AllowInvalidUTF8(false), AllowInvalidUTF8(true))` results in `AllowInvalidUTF8(true)` taking precedence.

Options that do not affect the operation in question are ignored. For example, passing `Expand` to `NewDecoder` does nothing.

The `WithIndent` and `WithIndentPrefix` flags configure the appearance of whitespace in the output. Their semantics are identical to the v1 [`Encoder.SetIndent`](https://pkg.go.dev/encoding/json#Encoder.SetIndent) method.

### Errors

Errors due to non-compliance with the JSON grammar are reported as `SyntacticError`. 
```go
type SyntacticError struct {
	// ByteOffset indicates that an error occurred after this byte offset.
	ByteOffset int64
	// JSONPointer indicates that an error occurred within this JSON value
	// as indicated using the JSON Pointer notation (see RFC 6901).
	JSONPointer string
	// Err is the underlying error.
	Err error // always non-nil
}
func (e *SyntacticError) Error() string
func (e *SyntacticError) Unwrap() error
```

Errors due to I/O are returned as an opaque error that unwrap to the original error returned by the failing `io.Reader.Read` or `io.Writer.Write` call.

## The v2 "json" package

The v2 "json" package provides functionality to marshal or unmarshal JSON data from or into Go value types. This package depends on "jsontext" to process JSON text and the "reflect" package to dynamically introspect Go values at runtime.

### Overview

The basic API consists of the following:
```go
package json // "encoding/json/v2"

func Marshal(in any, opts ...Options) (out []byte, err error)
func MarshalWrite(out io.Writer, in any, opts ...Options) error
func MarshalEncode(out *jsontext.Encoder, in any, opts ...Options) error

func Unmarshal(in []byte, out any, opts ...Options) error
func UnmarshalRead(in io.Reader, out any, opts ...Options) error
func UnmarshalDecode(in *jsontext.Decoder, out any, opts ...Options) error
```
The `Marshal` and `Unmarshal` functions mostly match the signature of the same functions in v1, however their behavior differs.

The `MarshalWrite` and `UnmarshalRead` functions are equivalent functionality that operate on an `io.Writer` and `io.Reader` instead of `[]byte`. The `UnmarshalRead` function consumes the entire input until `io.EOF` and reports an error if any invalid tokens appear after the end of the JSON value ([#36225](https://github.com/golang/go/issues/36225)).

The `MarshalEncode` and `UnmarshalDecode` functions are equivalent functionality that operate on an `*jsontext.Encoder` and `*jsontext.Decoder` instead of `[]byte`.

### Default behavior

The marshal and unmarshal logic in v2 is mostly identical to v1 with following changes:

| v1 | v2 |
| -- | -- |
| JSON object members are unmarshaled into a Go struct using a **case-insensitive name match**. | JSON object members are unmarshaled into a Go struct using a **case-sensitive name match**. |
| When marshaling a Go struct, a struct field marked as `omitempty` is omitted if **the field value is an empty Go value**, which is defined as false, 0, a nil pointer, a nil interface value, and any empty array, slice, map, or string. | When marshaling a Go struct, a struct field marked as `omitempty` is omitted if **the field value would encode as an empty JSON value**, which is defined as a JSON null, or an empty JSON string, object, or array ([more discussion](https://github.com/golang/go/discussions/63397#discussioncomment-7201224)). |
| The `string` option **does affect** Go bools and strings. | The `string` option **does not affect** Go bools and strings. |
| The `string` option **does not recursively affect** sub-values of the Go field value. | The `string` option **does recursively affect** sub-values of the Go field value. |
| The `string` option **sometimes accepts** a JSON null escaped within a JSON string. | The `string` option **never accepts** a JSON null escaped within a JSON string. |
| A nil Go slice is marshaled as a **JSON null**. | A nil Go slice is marshaled as an **empty JSON array** ([more discussion](https://github.com/golang/go/discussions/63397#discussioncomment-7201222)). |
| A nil Go map is marshaled as a **JSON null**. | A nil Go map is marshaled as an **empty JSON object** ([more discussion](https://github.com/golang/go/discussions/63397#discussioncomment-7201222)). |
| A Go array may be unmarshaled from a **JSON array of any length**. | A Go array must be unmarshaled from a **JSON array of the same length**. |
| A Go byte array is represented as a **JSON array of JSON numbers**. | A Go byte array is represented as a **JSON string containing the bytes in Base-64 encoding**. |
| `MarshalJSON` and `UnmarshalJSON` methods declared on a pointer receiver are **inconsistently called**. | `MarshalJSON` and `UnmarshalJSON` methods declared on a pointer receiver are **consistently called**. |
| A Go map is marshaled in a **deterministic order**. | A Go map is marshaled in a **non-deterministic order** ([more discussion](https://github.com/golang/go/discussions/63397#discussioncomment-7201221)). |
| JSON strings are encoded **with HTML-specific characters being escaped**. | JSON strings are encoded **without any characters being escaped** (unless necessary). |
| When marshaling, invalid UTF-8 within a Go string **are silently replaced**. | When marshaling, invalid UTF-8 within a Go string **results in an error**. |
| When unmarshaling, invalid UTF-8 within a JSON string **are silently replaced**. | When unmarshaling, invalid UTF-8 within a JSON string **results in an error**. |
| When marshaling, **an error does not occur** if the output JSON value contains objects with duplicate names. | When marshaling, **an error does occur** if the output JSON value contains objects with duplicate names. |
| When unmarshaling, **an error does not occur** if the input JSON value contains objects with duplicate names. | When unmarshaling, **an error does occur** if the input JSON value contains objects with duplicate names. |
| Unmarshaling a JSON null into a non-empty Go value **inconsistently clears the value or does nothing**. | Unmarshaling a JSON null into a non-empty Go value **always clears the value**. |
| Unmarshaling a JSON value into a non-empty Go value **follows inconsistent and bizarre behavior**. | Unmarshaling a JSON value into a non-empty Go value **always merges if the input is an object, and otherwise replaces** ([more discussion](https://github.com/golang/go/discussions/63397#discussioncomment-7201220)). |
| A `time.Duration` is represented as a **JSON number containing the decimal number of nanoseconds**. | A `time.Duration` is represented as a **JSON string containing the formatted duration** (e.g., "1h2m3.456s"). |
| Unmarshaling a JSON number into a Go float beyond its representation **results in an error**. | Unmarshaling a JSON number into a Go float beyond its representation **uses the closest representable value** (e.g., `±math.MaxFloat`). |
| A Go struct with only unexported fields **can be serialized**. | A Go struct with only unexported fields **cannot be serialized**. |
| A Go struct that embeds an unexported struct type **can sometimes be serialized**. | A Go struct that embeds an unexported struct type **cannot be serialized**. |

See [here for details](https://github.com/go-json-experiment/json/blob/master/diff_test.go) about every change.

Every behavior change will be configurable through options, which will be a critical part of how we achieve v1-to-v2 interoperability.
See [here for more discussion](https://github.com/golang/go/discussions/63397#discussioncomment-7201218).

### Struct tag options

Similar to v1, v2 also supports customized representation of Go struct fields through the use of struct tags. As before, the `json` tag will be used. The following tag options are supported:

- **omitzero**: When marshaling, the "omitzero" option specifies that the struct field should be omitted if the field value is zero, as determined by the "IsZero() bool" method, if present, otherwise based on whether the field is the zero Go value (per [`reflect.Value.IsZero`](https://pkg.go.dev/reflect#Value.IsZero)). This option has no effect when unmarshaling. ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-package-OmitFields))

    - **New in v2**. The inability to omit an empty struct is a frequently cited issue in v1. This feature is intended to provide a general way to accomplish that goal ([#11939](https://github.com/golang/go/issues/11939), [#22480](https://github.com/golang/go/issues/22480), [#50480](https://github.com/golang/go/issues/50480), [#29310](https://github.com/golang/go/issues/29310), [#52803](https://github.com/golang/go/issues/52803), [#45669](https://github.com/golang/go/issues/45669)).

- **omitempty**: When marshaling, the "omitempty" option specifies that the struct field should be omitted if the field value would have been encoded as a JSON null, empty string, empty object, or empty array. This option has no effect when unmarshaling. ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-package-OmitFields))

    - **Changed in v2**. In v1, the "omitempty" option was narrowly defined as only omitting a field if it is a Go false, 0, a nil pointer, a nil interface value, and any empty array, slice, map, or string. In v2, it has been redefined in terms of the JSON type system, rather than the Go type system. They are practically equivalent except for Go bools and numbers, for which the "omitzero" option can be used instead ([more discussion](https://github.com/golang/go/discussions/63397#discussioncomment-7201224)).

- **string**: The "string" option specifies that `StringifyNumbers` be set when marshaling or unmarshaling a struct field value. This causes numeric types to be encoded as a JSON number within a JSON string, and to be decoded from either a JSON number or a JSON string containing a JSON number. This extra level of encoding is often necessary since many JSON parsers cannot precisely represent 64-bit integers.

    - **Changed in v2**. In v1, the "string" option applied to certain types where use of a JSON string did not make sense (e.g., a bool) and could not be applied recursively (e.g., a slice of integers). In v2, this feature only applies to numeric types and applies recursively. 

- **nocase**: When unmarshaling, the "nocase" option specifies that if the JSON object name does not exactly match the JSON name for any of the struct fields, then it attempts to match the struct field using a case-insensitive match that also ignores dashes and underscores. ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-package-CaseSensitivity))

    - **New in v2**. Since v2 no longer performs a case-insensitive match of JSON object names, this option provides a means to opt-into the v1-like behavior. However, the case-insensitive match is altered relative to v1 in that it also ignores dashes and underscores. This makes the feature more broadly useful for JSON objects with different naming conventions to be unmarshaled. For example, "fooBar", "FOO_BAR", or "foo-bar" will all match with a field named "FooBar".

- **inline**: The "inline" option specifies that the JSON object representation of this field is to be promoted as if it were specified in the parent struct. It is the JSON equivalent of Go struct embedding. A Go embedded field is implicitly inlined unless an explicit JSON name is specified. The inlined field must be a Go struct that does not implement `Marshaler` or `Unmarshaler`. Inlined fields of type `jsontext.Value` and `map[string]T` are called “inlined fallbacks”, as they can represent all possible JSON object members not directly handled by the parent struct. Only one inlined fallback field may be specified in a struct, while many non-fallback fields may be specified. This option must not be specified with any other tag option. ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-package-InlinedFields))

    - **New in v2**. Inlining is an explicit way to embed a JSON object within another JSON object without relying on Go struct embedding. The feature is capable of inlining Go maps and `jsontext.Value` ([#6213](https://github.com/golang/go/issues/6213)).

- **unknown**: The "unknown" option is a specialized variant of the inlined fallback to indicate that this Go struct field contains any number of “unknown” JSON object members. The field type must be a `jsontext.Value`, `map[string]T`. If `DiscardUnknownMembers` is specified when marshaling, the contents of this field are ignored. If `RejectUnknownMembers` is specified when unmarshaling, any unknown object members are rejected even if a field exists with the "unknown" option. This option must not be specified with any other tag option. ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-package-UnknownMembers))

    - **New in v2**. The "inline" feature technically provides a way to preserve unknown member ([#22533](https://github.com/golang/go/issues/22533)). However, the "inline" feature alone does not semantically tell us whether this field is meant to store unknown members. The "unknown" option gives us this extra bit of information so that we can cooperate with options that affect unknown membership.

- **format**: The "format" option specifies a format flag used to specialize the formatting of the field value. The option is a key-value pair specified as "format:value" where the value must be either a literal consisting of letters and numbers (e.g., "format:RFC3339") or a single-quoted string literal (e.g., "format:'2006-01-02'"). The interpretation of the format flag is determined by the struct field type. ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-package-FormatFlags))

    - **New in v2**. The "format" option provides a general way to customize formatting of arbitrary types.

    - `[]byte` and `[N]byte` types accept "format" values of either "base64", "base64url", "base32", "base32hex", "base16", or "hex", where it represents the binary bytes as a JSON string encoded using the specified format in RFC 4648. It may also be "array" to treat the slice or array as a JSON array of numbers. The "array" format exists for backwards compatibility since the default representation of an array of bytes now uses Base-64.

    - `float32` and `float64` types accept a "format" value of "nonfinite", where NaN and infinity are represented as JSON strings.

    - Slice types accept a "format" value of "emitnull" to marshal a nil slice as a JSON null instead of an empty JSON array. ([more discussion](https://github.com/golang/go/discussions/63397#discussioncomment-7201222)).

    - Map types accept a "format" value of "emitnull" to marshal a nil map as a JSON null instead of an empty JSON object. ([more discussion](https://github.com/golang/go/discussions/63397#discussioncomment-7201222)).

    - The `time.Time` type accepts a "format" value which may either be a Go identifier for one of the format constants (e.g., "RFC3339") or the format string itself to use with `time.Time.Format` or `time.Parse` ([#21990](https://github.com/golang/go/issues/21990)). It can also be "unix", "unixmilli", "unixmicro", or "unixnano" to be represented as a decimal number reporting the number of seconds (or milliseconds, etc.) since the Unix epoch.

    - The `time.Duration` type accepts a "format" value of "sec", "milli", "micro", or "nano" to represent it as the number of seconds (or milliseconds, etc.) formatted as a JSON number. This exists for backwards compatibility since the default representation now uses a string representation (e.g., "53.241s"). If the format is "base60", it is encoded as a JSON string using the "H:MM:SS.SSSSSSSSS" representation.

The "omitzero" and "omitempty" options are similar. The former is defined in terms of the Go type system, while the latter in terms of the JSON type system. Consequently they behave differently in some circumstances. For example, only a nil slice or map is omitted under "omitzero", while an empty slice or map is omitted under "omitempty" regardless of nilness. The "omitzero" option is useful for types with a well-defined zero value (e.g., `netip.Addr`) or have an `IsZero` method (e.g., `time.Time`).

### Type-specified customization

Go types may customize their own JSON representation by implementing certain interfaces that the "json" package knows to look for:
```go
type MarshalerV1 interface {
	MarshalJSON() ([]byte, error)
}
type MarshalerV2 interface {
	MarshalJSONV2(*jsontext.Encoder, Options) error
}

type UnmarshalerV1 interface {
	UnmarshalJSON([]byte) error
}
type UnmarshalerV2 interface {
	UnmarshalJSONV2(*jsontext.Decoder, Options) error
}
```
The v1 interfaces are supported in v2 to provide greater degrees of backward compatibility. If a type implements both v1 and v2 interfaces, the v2 variant takes precedence. The v2 interfaces operate in a purely streaming manner. This API can provide dramatic performance improvements. For example, switching from `UnmarshalJSON` to `UnmarshalJSONV2` for `spec.Swagger` resulted in an [~40x performance improvement](https://github.com/kubernetes/kube-openapi/issues/315#issuecomment-1240030015).

### Caller-specified customization

In addition to Go types being able to specify their own JSON representation, the caller of the marshal or unmarshal functionality can also specify their own JSON representation for specific Go types ([#5901](https://github.com/golang/go/issues/5901)). Caller-specified customization takes precedence over type-specified customization.

```go
// SkipFunc may be returned by MarshalFuncV2 and UnmarshalFuncV2 functions.
// Any function that returns SkipFunc must not cause observable side effects
// on the provided Encoder or Decoder.
const SkipFunc = jsonError("skip function")

// Marshalers holds a list of functions that may override the marshal behavior
// of specific types. Populate WithMarshalers to use it.
// A nil *Marshalers is equivalent to an empty list.
type Marshalers struct { /* no exported fields */ }

// NewMarshalers constructs a flattened list of marshal functions.
// If multiple functions in the list are applicable for a value of a given type,
// then those earlier in the list take precedence over those that come later.
// If a function returns SkipFunc, then the next applicable function is called,
// otherwise the default marshaling behavior is used.
//
// For example:
//
//	m1 := NewMarshalers(f1, f2)
//	m2 := NewMarshalers(f0, m1, f3)     // equivalent to m3
//	m3 := NewMarshalers(f0, f1, f2, f3) // equivalent to m2
func NewMarshalers(ms ...*Marshalers) *Marshalers

// MarshalFuncV1 constructs a type-specific marshaler that
// specifies how to marshal values of type T.
func MarshalFuncV1[T any](fn func(T) ([]byte, error)) *Marshalers

// MarshalFuncV2 constructs a type-specific marshaler that
// specifies how to marshal values of type T.
// The function is always provided with a non-nil pointer value
// if T is an interface or pointer type.
func MarshalFuncV2[T any](fn func(*jsontext.Encoder, T, Options) error) *Marshalers

// Unmarshalers holds a list of functions that may override the unmarshal behavior
// of specific types. Populate WithUnmarshalers to use it.
// A nil *Unmarshalers is equivalent to an empty list.
type Unmarshalers struct { /* no exported fields */ }

// NewUnmarshalers constructs a flattened list of unmarshal functions.
// It operates in a similar manner as NewMarshalers.
func NewUnmarshalers(us ...*Unmarshalers) *Unmarshalers

// UnmarshalFuncV1 constructs a type-specific unmarshaler that
// specifies how to unmarshal values of type T.
func UnmarshalFuncV1[T any](fn func([]byte, T) error) *Unmarshalers

// UnmarshalFuncV2 constructs a type-specific unmarshaler that
// specifies how to unmarshal values of type T.
// T must be an unnamed pointer or an interface type.
// The function is always provided with a non-nil pointer value.
func UnmarshalFuncV2[T any](fn func(*jsontext.Decoder, T, Options) error) *Unmarshalers
```

The `MarshalFuncV1` and `UnmarshalFuncV1` functions can always be implemented in terms of the v2 variants, which calls into question their utility. There are several reasons for providing them:

1. To maintain symmetry and consistency with the method interfaces (which must provide both v1 and v2 variants).

2. To make it interoperate well with existing functionality that operate on the v1 signature. For example, to integrate the v2 "json" package with proper JSON serialization of protocol buffers, one could construct a type-specific marshaler using `json.MarshalFuncV1(protojson.Marshal)`, where [`protojson.Marshal`](https://pkg.go.dev/google.golang.org/protobuf/encoding/protojson#Marshal) provides the JSON representation for all types that implement [`proto.Message`](https://pkg.go.dev/google.golang.org/protobuf/proto#Message) ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-package-ProtoJSON)).

Caller-specified customization is a powerful feature. For example:
- It can be used to marshal Go errors ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-WithMarshalers-Errors)).
- It can be used to preserve the raw representation of JSON numbers ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-WithUnmarshalers-RawNumber)). Note that v2 does not have the v1 `RawNumber` type.
- It can be used to preserve the input offset of JSON values for error reporting purposes ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-WithUnmarshalers-RecordOffsets)).

### Options

Options may be specified that configure how marshal and unmarshal operates:
```go
// Options configure Marshal, MarshalWrite, MarshalEncode,
// Unmarshal, UnmarshalRead, and UnmarshalDecode with specific features.
// Each function takes in a variadic list of options, where properties set
// in latter options override the value of previously set properties.
//
// Options represent either a singular option or a set of options.
// It can be functionally thought of as a Go map of option properties
// (even though the underlying implementation avoids Go maps for performance).
//
// The constructors (e.g., Deterministic) return a singular option value:
//	opt := Deterministic(true)
// which is analogous to creating a single entry map:
//	opt := Options{"Deterministic": true}
//
// JoinOptions composes multiple options values to together:
//	out := JoinOptions(opts...)
// which is analogous to making a new map and copying the options over:
//	out := make(Options)
//	for _, m := range opts {
//		for k, v := range m {
//			out[k] = v
//		}
//	}
//
// GetOption looks up the value of options parameter:
//	v, ok := GetOption(opts, Deterministic)
// which is analogous to a Go map lookup:
//	v, ok := opts["Deterministic"]
//
// There is a single Options type, which is used with both marshal and unmarshal.
// Options that do not affect a particular operation are ignored.
type Options = jsonopts.Options

// StringifyNumbers specifies that numeric Go types should be marshaled as
// a JSON string containing the equivalent JSON number value.
// When unmarshaling, numeric Go types can be parsed from either a JSON number
// or a JSON string containing the JSON number without any surrounding whitespace.
func StringifyNumbers(v bool) Options // affects marshal and unmarshal

// Deterministic specifies that the same input value will be serialized
// as the exact same output bytes. Different processes of
// the same program will serialize equal values to the same bytes,
// but different versions of the same program are not guaranteed
// to produce the exact same sequence of bytes.
func Deterministic(v bool) Options // affects marshal only

// FormatNilMapAsNull specifies that a nil Go map should marshal as a
// JSON null instead of the default representation as an empty JSON object.
func FormatNilMapAsNull(v bool) Options // affects marshal only

// FormatNilSliceAsNull specifies that a nil Go slice should marshal as a
// JSON null instead of the default representation as an empty JSON array
// (or an empty JSON string in the case of ~[]byte).
func FormatNilSliceAsNull(v bool) Options // affects marshal only

// MatchCaseInsensitiveNames specifies that JSON object members are matched
// against Go struct fields using a case-insensitive match of the name.
func MatchCaseInsensitiveNames(v bool) Options // affects marshal and unmarshal

// DiscardUnknownMembers specifies that marshaling should ignore any
// JSON object members stored in Go struct fields dedicated to storing
// unknown JSON object members.
func DiscardUnknownMembers(v bool) Options // affects marshal only

// RejectUnknownMembers specifies that unknown members should be rejected
// when unmarshaling a JSON object, regardless of whether there is a field
// to store unknown members.
func RejectUnknownMembers(v bool) Options // affects unmarshal only

// WithMarshalers specifies a list of type-specific marshalers to use,
// which can be used to override the default marshal behavior
// for values of particular types.
func WithMarshalers(v *Marshalers) Options // affects marshal only

// WithUnmarshalers specifies a list of type-specific unmarshalers to use,
// which can be used to override the default unmarshal behavior
// for values of particular types.
func WithUnmarshalers(v *Unmarshalers) Options // affects unmarshal only

// JoinOptions coalesces the provided list of options into a single Options.
// Properties set in latter options override the value of previously set properties.
func JoinOptions(srcs ...Options) Options

// GetOption returns the value stored in opts with the provided constructor,
// reporting whether the value is present.
func GetOption[T any](opts Options, constructor func(T) Options) (T, bool)
```
The `Options` type is a type alias to an internal type that is an interface type with no exported methods. It is used simply as a marker type for options declared in the "json" and "jsontext" package. This is exactly the same `Options` type as the one in the "jsontext" package.

The same `Options` type is used for both `Marshal` and `Unmarshal` as some options affect both operations.

The `MarshalJSONV2`, `UnmarshalJSONV2`, `MarshalFuncV2`, and `UnmarshalFuncV2` methods and functions take in a singular `Options` value instead of a variadic list because the `Options` type can represent a set of options. The caller (which is the "json" package) can coalesce a list of options before calling the user-specified method or function. Being given a single `Options` value is more ergonomic for the user as there is only one options value to introspect with `GetOption`.

While the `JoinOptions` constructor technically removes the need for `NewEncoder`, `NewDecoder`, `Marshal`, and `Unmarshal` from taking in a variadic list of options, it is more ergonomic for it to be variadic as the user can more readily specify a list of options without needing to call `JoinOptions` first.

### Errors

Errors due to the inability to correlate JSON data with Go data are reported as `SemanticError`. 
```go
type SemanticError struct {
	// ByteOffset indicates that an error occurred after this byte offset.
	ByteOffset int64
	// JSONPointer indicates that an error occurred within this JSON value
	// as indicated using the JSON Pointer notation (see RFC 6901).
	JSONPointer string

	// JSONKind is the JSON kind that could not be handled.
	JSONKind Kind // may be zero if unknown
	// GoType is the Go type that could not be handled.
	GoType reflect.Type // may be nil if unknown

	// Err is the underlying error.
	Err error // may be nil
}
func (e *SemanticError) Error() string
func (e *SemanticError) Unwrap() error
```

## Experimental implementation

The draft proposal has been implemented by the [`github.com/go-json-experiment/json`](https://pkg.go.dev/github.com/go-json-experiment/json) module.

### Stability

We have confidence in the correctness and performance of the module as it has been used internally at [Tailscale](https://tailscale.com/) in various production services. However, the module is an experiment and **breaking changes are expected to occur** based on feedback in this discussion, it should not be depended upon by publicly available code, otherwise we can run into situations where large programs fail to build.

Consider the following situation:
- Program P depends on modules A and B.
- Module A depends on `go-json-experiment/json@v0.5.0`.
- Module B depends on `go-json-experiment/json@v0.8.0`.
- Let’s suppose a breaking change occurs between `v0.5.0` and `v0.8.0`.
- [MVS](https://research.swtch.com/vgo-mvs) dictates that `v0.8.0` be selected to build program P.
- However, the use of `v0.8.0` breaks module A since it is using the API for `v0.5.0`, which is not compatible.

If open source code does use `go-json-experiment`, we recommend that use of it be guarded by a build tag or the entire module be forked and vendored as a dependency.

### Performance

Due to a combination of both a more efficient implementation and also changes to the external API to better support performance, the experimental v2 implementation is generally as fast or slightly faster for marshaling and dramatically faster for unmarshaling.

See the [benchmarks for results](https://github.com/go-json-experiment/jsonbench).