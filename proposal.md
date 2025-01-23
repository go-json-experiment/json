# Proposal Details

This is a formal proposal for the addition of "encoding/json/v2" and "encoding/json/jsontext" packages that has previously been discussed in #63397.

This focuses on just the newly added API. An argument justifying the need for a v2 package can be found in prior discussion. Alternatively, you can watch the GopherCon talk entitled ["The Future of JSON in Go"](https://www.youtube.com/watch?v=avilmOcHKHE).

Most of the API proposal below is copied from the discussion.
If you've already read the discussion and only want to know
what changed relative to the discussion,
skip over to the ["Changes from discussion"](#changes-from-discussion) section.

In general, we propose the addition of the following:
* Package "encoding/json/jsontext", which handles processing of JSON purely at a syntactic layer (with no dependencies on Go reflection). This is a lower-level
package that most users will not use, but still sufficiently useful to expose
as a standalone package.
* Package "encoding/json/v2", which will serve as the second major version of the v1 "encoding/json" package. It is implemented in terms of "jsontext".
* Additional API in v1 "encoding/json" to provide inter-operability with v2.
It is implemented in terms of "json/v2".

Thank you to everyone who has been involved with the discussion, design review, code review, etc. This proposal is better off because of all your feedback.

## Package "encoding/json/jsontext"

The `jsontext` package provides functionality to process JSON purely according to the grammar.

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
func (*Decoder) PeekKind() Kind // MUSTDO: Should return an error?
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
func (v Value) IsValid(opts ...Options) bool
func (v *Value) Format(opts ...Options) error
func (v *Value) Compact(opts ...Options) error
func (v *Value) Indent(opts ...Options) error
func (v *Value) Canonicalize(opts ...Options) error
func (v Value) MarshalJSON() ([]byte, error)
func (v *Value) UnmarshalJSON(b []byte) error
func (v Value) Kind() Kind // never ']' or '}' if valid
```

By default, `IsValid` validates according to RFC 7493, but accepts options to
validate according to looser guarantees (such as allowing duplicate names).

The `Format` method formats the value according to the specified options.
The `Compact` and `Indent` methods operate similar to the v1 `Compact` and `Indent` function.
The `Canonicalize` method canonicalizes the JSON value according to the JSON Canonicalization Scheme as defined in [RFC 8785](https://www.rfc-editor.org/rfc/rfc8785).
The `Compact`, `Indent`, and `Canonicalize` each called `Format` with a default list
of options. The caller may provide more options to override the defaults.

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

### Formatting

Some top-level functions are provided for formatting JSON values and strings.

```go
// AppendFormat formats the JSON value in src and appends it to dst
// according to the specified options.
// See [Value.Format] for more details about the formatting behavior.
func AppendFormat(dst, src []byte, opts ...Options) ([]byte, error)

// AppendQuote appends a double-quoted JSON string literal representing src
// to dst and returns the extended buffer.
func AppendQuote[Bytes ~[]byte | ~string](dst []byte, src Bytes) ([]byte, error)

// AppendUnquote appends the decoded interpretation of src as a
// double-quoted JSON string literal to dst and returns the extended buffer.
// The input src must be a JSON string without any surrounding whitespace.
func AppendUnquote[Bytes ~[]byte | ~string](dst []byte, src Bytes) ([]byte, error)
```

### Encoder and Decoder

The `Encoder` and `Decoder` types provide the functionality for encoding to or decoding from an `io.Writer` or an `io.Reader`. An `Encoder` or `Decoder` can be constructed with`NewEncoder` or `NewDecoder` using default options.

The `Encoder` is a streaming encoder from raw JSON tokens and values. It is used to write a stream of top-level JSON values, each terminated with a newline character.
```go
type Encoder struct { /* no exported fields */ }

func NewEncoder(w io.Writer, opts ...Options) *Encoder
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

func NewDecoder(r io.Reader, opts ...Options) *Decoder
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
func (e *Encoder) StackIndex(i int) (Kind, int64)
func (d *Decoder) StackIndex(i int) (Kind, int64)

// StackPointer returns a JSON Pointer (RFC 6901) to the most recently handled value.
func (e *Encoder) StackPointer() Pointer
func (d *Decoder) StackPointer() Pointer
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

// CanonicalizeRawFloats specifies that when encoding a raw JSON floating-pointer number
// (i.e., a number with a fraction or exponent) in a [Token] or [Value],
// the number is canonicalized according to RFC 8785, section 3.2.2.3.
func CanonicalizeRawFloats(v bool) Options // affect encode only

// CanonicalizeRawInts specifies that when encoding a raw JSON integer number
// (i.e., a number without a fraction and exponent) in a [Token] or [Value],
// the number is canonicalized according to RFC 8785, section 3.2.2.3.
func CanonicalizeRawInts(v bool) Options // affect encode only

// PreserveRawStrings specifies that when encoding a raw JSON string
// in a [Token] or [Value], pre-escaped sequences in a JSON string
// are preserved to the output.
func PreserveRawStrings(v bool) Options // affect encode only

// ReorderRawObjects specifies that when encoding a raw JSON object in a [Value],
// the object members are reordered according to RFC 8785, section 3.2.3.
func ReorderRawObjects(v bool) Options // affect encode only

// EscapeForHTML specifies that '<', '>', and '&' characters within JSON strings
// should be escaped as a hexadecimal Unicode codepoint (e.g., \u003c)
// so that the output is safe to embed within HTML.
func EscapeForHTML(v bool) Options // affects encode only

// EscapeForJS specifies that U+2028 and U+2029 characters within JSON strings
// should be escaped as a hexadecimal Unicode codepoint (e.g., \u2028)
// so that the output is valid to embed within JavaScript.
// See RFC 8259, section 12.
func EscapeForJS(v bool) Options // affects encode only

// Multiline specifies that the JSON output should be expanded, where
// every JSON object member or JSON array element appears on a new, indented line
// according to the nesting depth.
// If an indent is not already specified, then it defaults to using "\t".
func Multiline(v bool) Options // affects encode only

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

// SpaceAfterColon specifies that the JSON output should emit a space character
// after each colon separator following a JSON object name.
func SpaceAfterColon(v bool) Options // affect encode only

// SpaceAfterComma specifies that the JSON output should emit a space character
// after each comma separator following a JSON object value or array element.
func SpaceAfterComma(v bool) Options // affect encode only
```
The `Options` type is a type alias to an internal type that is an interface type with no exported methods. It is used simply as a marker type for options declared in the "json" and "jsontext" packages.

Latter option specified in the variadic list passed to `NewEncoder` and `NewDecoder` takes precedence over prior option values. For example, `NewEncoder(AllowInvalidUTF8(false), AllowInvalidUTF8(true))` results in `AllowInvalidUTF8(true)` taking precedence.

Options that do not affect the operation in question are ignored. For example, passing `Multiline` to `NewDecoder` does nothing.

The `WithIndent` and `WithIndentPrefix` flags configure the appearance of whitespace in the output. Their semantics are identical to the v1 [`Encoder.SetIndent`](https://pkg.go.dev/encoding/json#Encoder.SetIndent) method.

### Errors

Errors due to non-compliance with the JSON grammar are reported as `SyntacticError`. 
```go
type SyntacticError struct {
	// ByteOffset indicates that an error occurred after this byte offset.
	ByteOffset int64
	// JSONPointer indicates that an error occurred within this JSON value
	// as indicated using the JSON Pointer notation (see RFC 6901).
	JSONPointer Pointer
	// Err is the underlying error.
	Err error // always non-nil
}
func (e *SyntacticError) Error() string
func (e *SyntacticError) Unwrap() error
```

Errors due to I/O are returned as an opaque error that unwrap to the original error returned by the failing `io.Reader.Read` or `io.Writer.Write` call.

```go
// ErrDuplicateName indicates that a JSON token could not be
// encoded or decoded because it results in a duplicate JSON object name.
var ErrDuplicateName = errors.New("duplicate object member name")

// ErrNonStringName indicates that a JSON token could not be
// encoded or decoded because it is not a string,
// as required for JSON object names according to RFC 8259, section 4.
var ErrNonStringName = errors.New("object member name must be a string")
```
`ErrDuplicateName` and `ErrNonStringName` are sentinel errors that are
returned while being wrapped within a `SyntacticError`.

```go
type Pointer string
func (p Pointer) IsValid() bool
func (p Pointer) AppendToken(tok string) Pointer
func (p Pointer) Parent() Pointer
func (p1 Pointer) Contains(p2 Pointer) bool
func (p Pointer) LastToken() string
func (p Pointer) Tokens() iter.Seq[string]
```
`Pointer` is a named type representing a JSON Pointer (RFC 6901) and
references a particular JSON value relative a top-level JSON value.
It is primarily used for error reporting, but it's utility could be expanded
in the future (e.g. extracting or modifying a portion of a `Value`
by `Pointer` reference alone).

## Package "encoding/json/v2"

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

- In v1, JSON object members are unmarshaled into a Go struct using a
  case-insensitive name match with the JSON name of the fields.
  In contrast, v2 matches fields using an exact, case-sensitive match.
  The `MatchCaseInsensitiveNames` and `jsonv1.MatchCaseSensitiveDelimiter`
  options control this behavior difference. To explicitly specify a Go struct
  field to use a particular name matching scheme, either the `nocase`
  or the `strictcase` field option can be specified.

- In v1, when marshaling a Go struct, a field marked as `omitempty`
  is omitted if the field value is an "empty" Go value, which is defined as
  false, 0, a nil pointer, a nil interface value, and
  any empty array, slice, map, or string. In contrast, v2 redefines
  `omitempty` to omit a field if it encodes as an "empty" JSON value,
  which is defined as a JSON null, or an empty JSON string, object, or array.
  The `jsonv1.OmitEmptyWithLegacyDefinition` option controls this behavior difference.
  Note that `omitempty` behaves identically in both v1 and v2 for a
  Go array, slice, map, or string (assuming no user-defined MarshalJSON method
  The `jsonv1.StringifyWithLegacySemantics` option controls this behavior difference.
  overrides the default representation). Existing usages of `omitempty` on a
  Go bool, number, pointer, or interface value should migrate to specifying
  `omitzero` instead (which is identically supported in both v1 and v2).

- In v1, a Go struct field marked as `string` can be used to quote a
  Go string, bool, or number as a JSON string. It does not recursively
  take effect on composite Go types. In contrast, v2 restricts
  the `string` option to only quote a Go number as a JSON string.
  It does recursively take effect on Go numbers within a composite Go type.

- In v1, a nil Go slice or Go map are marshaled as a JSON null.
  In contrast, v2 marshals a nil Go slice or Go map as
  an empty JSON array or JSON object, respectively.
  The `FormatNilSliceAsNull` and `FormatNilMapAsNull` options
  control this behavior difference. To explicitly specify a Go struct field
  to use a particular representation for nil, either the `format:emitempty`
  or `format:emitnull` field option can be specified.

- In v1, a Go array may be unmarshaled from a JSON array of any length.
  In contrast, in v2 a Go array must be unmarshaled from a JSON array
  of the same length, otherwise it results in an error.
  The `jsonv1.UnmarshalArrayFromAnyLength` option controls this behavior difference.

- In v1, a Go byte array is represented as a JSON array of JSON numbers.
  In contrast, in v2 a Go byte array is represented as a Base64-encoded JSON string.
  The `jsonv1.FormatBytesWithLegacySemantics` option controls this behavior difference.
  To explicitly specify a Go struct field to use a particular representation,
  either the `format:array` or `format:base64` field option can be specified.

- In v1, MarshalJSON methods declared on a pointer receiver are only called
  if the Go value is addressable. In contrast, in v2 a MarshalJSON method
  is always callable regardless of addressability.
  The `jsonv1.CallMethodsWithLegacySemantics` option controls this behavior difference.

- In v1, MarshalJSON and UnmarshalJSON methods are never called for Go map keys.
  In contrast, in v2 a MarshalJSON or UnmarshalJSON method is eligible for
  being called for Go map keys.
  The `jsonv1.CallMethodsWithLegacySemantics` option controls this behavior difference.

- In v1, a Go map is marshaled in a deterministic order.
  In contrast, in v2 a Go map is marshaled in a non-deterministic order.
  The `Deterministic` option controls this behavior difference.

- In v1, JSON strings are encoded with HTML-specific or JavaScript-specific
  characters being escaped. In contrast, in v2 JSON strings use the minimal
  encoding and only escape if required by the JSON grammar.
  The `jsontext.EscapeForHTML` and `jsontext.EscapeForJS` options
  control this behavior difference.

- In v1, bytes of invalid UTF-8 within a string are silently replaced with
  the Unicode replacement character. In contrast, in v2 the presence of
  invalid UTF-8 results in an error. The `jsontext.AllowInvalidUTF8` option
  controls this behavior difference.

- In v1, a JSON object with duplicate names is permitted.
  In contrast, in v2 a JSON object with duplicate names results in an error.
  The `jsontext.AllowDuplicateNames` option controls this behavior difference.

- In v1, when unmarshaling a JSON null into a non-empty Go value it will
  inconsistently either zero out the value or do nothing.
  In contrast, in v2 unmarshaling a JSON null will consistently and always
  zero out the underlying Go value. The `jsonv1.MergeWithLegacySemantics` option
  controls this behavior difference.

- In v1, when unmarshaling a JSON value into a non-zero Go value,
  it merges into the original Go value for array elements, slice elements,
  struct fields (but not map values),
  pointer values, and interface values (only if a non-nil pointer).
  In contrast, in v2 unmarshal merges into the Go value
  for struct fields, map values, pointer values, and interface values.
  In general, the v2 semantic merges when unmarshaling a JSON object,
  otherwise it replaces the value. The `jsonv1.MergeWithLegacySemantics` option
  controls this behavior difference.

- In v1, a `time.Duration` is represented as a JSON number containing
  the decimal number of nanoseconds. In contrast, in v2 a `time.Duration`
  is represented as a JSON string containing the formatted duration
  (e.g., "1h2m3.456s") according to `time.Duration.String`.
  The `jsonv1.FormatTimeWithLegacySemantics` option controls this behavior difference.
  To explicitly specify a Go struct field to use a particular representation,
  either the `format:nano` or `format:units` field option can be specified.

- In v1, errors are never reported at runtime for Go struct types
  that have some form of structural error (e.g., a malformed tag option).
  In contrast, v2 reports a runtime error for Go types that are invalid
  as they relate to JSON serialization. For example, a Go struct
  with only unexported fields cannot be serialized.
  The `jsonv1.ReportErrorsWithLegacySemantics` option controls this behavior difference.

### Struct tag options

Similar to v1, v2 also supports customized representation of Go struct fields through the use of struct tags. As before, the `json` tag will be used. The following tag options are supported:

- **omitzero**: When marshaling, the "omitzero" option specifies that the struct field should be omitted if the field value is zero, as determined by the "IsZero() bool" method, if present, otherwise based on whether the field is the zero Go value (per [`reflect.Value.IsZero`](https://pkg.go.dev/reflect#Value.IsZero)). This option has no effect when unmarshaling. ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-package-OmitFields))

    - **New in v2**, but has already been backported to v1 (see #45669) in Go 1.24.

- **omitempty**: When marshaling, the "omitempty" option specifies that the struct field should be omitted if the field value would have been encoded as a JSON null, empty string, empty object, or empty array. This option has no effect when unmarshaling. ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-package-OmitFields))

    - **Changed in v2**. In v1, the "omitempty" option was narrowly defined as only omitting a field if it is a Go false, 0, a nil pointer, a nil interface value, and any empty array, slice, map, or string. In v2, it has been redefined in terms of the JSON type system, rather than the Go type system. They are practically equivalent except for Go bools and numbers, for which the "omitzero" option can be used instead.

- **string**: The "string" option specifies that `StringifyNumbers` be set when marshaling or unmarshaling a struct field value. This causes numeric types to be encoded as a JSON number within a JSON string, and to be decoded from either a JSON number or a JSON string containing a JSON number. This extra level of encoding is often necessary since many JSON parsers cannot precisely represent 64-bit integers.

    - **Changed in v2**. In v1, the "string" option applied to certain types where use of a JSON string did not make sense (e.g., a bool) and could not be applied recursively (e.g., a slice of integers). In v2, this feature only applies to numeric types and applies recursively.

- **nocase**: When unmarshaling, the "nocase" option specifies that if the JSON object name does not exactly match the JSON name for any of the struct fields, then it attempts to match the struct field using a case-insensitive match that also ignores dashes and underscores. ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-package-CaseSensitivity))

    - **New in v2**. Since v2 no longer performs a case-insensitive match of JSON object names, this option provides a means to opt-into the v1-like behavior. However, the case-insensitive match is altered relative to v1 in that it also ignores dashes and underscores. This makes the feature more broadly useful for JSON objects with different naming conventions to be unmarshaled. For example, "fooBar", "FOO_BAR", or "foo-bar" will all match with a field named "FooBar".

- **strictcase**: When unmarshaling, the "strictcase" option specifies that the JSON object name must exactly match the JSON name for the struct field. This takes precedence even if MatchCaseInsensitiveNames is set to true. This cannot be specified together with the "nocase" option.

    - **New in v2** to provide an explicit author-specified way to prevent `MatchCaseInsensitiveNames`
    from taking effect on a particular field.

- **inline**: The "inline" option specifies that the JSON object representation of this field is to be promoted as if it were specified in the parent struct. It is the JSON equivalent of Go struct embedding. A Go embedded field is implicitly inlined unless an explicit JSON name is specified. The inlined field must be a Go struct that does not implement `Marshaler` or `Unmarshaler`. Inlined fields of type `jsontext.Value` and `map[~string]T` are called “inlined fallbacks”, as they can represent all possible JSON object members not directly handled by the parent struct. Only one inlined fallback field may be specified in a struct, while many non-fallback fields may be specified. This option must not be specified with any other tag option. ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-package-InlinedFields))

    - **New in v2**. Inlining is an explicit way to embed a JSON object within another JSON object without relying on Go struct embedding. The feature is capable of inlining Go maps and `jsontext.Value` ([#6213](https://github.com/golang/go/issues/6213)).

- **unknown**: The "unknown" option is a specialized variant of the inlined fallback to indicate that this Go struct field contains any number of “unknown” JSON object members. The field type must be a `jsontext.Value`, `map[~string]T`. If `DiscardUnknownMembers` is specified when marshaling, the contents of this field are ignored. If `RejectUnknownMembers` is specified when unmarshaling, any unknown object members are rejected even if a field exists with the "unknown" option. This option must not be specified with any other tag option. ([example](https://pkg.go.dev/github.com/go-json-experiment/json#example-package-UnknownMembers))

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
type Marshaler interface {
	MarshalJSON() ([]byte, error)
}
type MarshalerTo interface {
	MarshalJSONTo(*jsontext.Encoder, Options) error
}

type Unmarshaler interface {
	UnmarshalJSON([]byte) error
}
type UnmarshalerFrom interface {
	UnmarshalJSONFrom(*jsontext.Decoder, Options) error
}
```
The v1 interfaces are supported in v2 to provide greater degrees of backward compatibility. If a type implements both v1 and v2 interfaces, the v2 variant takes precedence. The v2 interfaces operate in a purely streaming manner. This API can provide dramatic performance improvements. For example, switching from `UnmarshalJSON` to `UnmarshalJSONFrom` for `spec.Swagger` resulted in an [~40x performance improvement](https://github.com/kubernetes/kube-openapi/issues/315#issuecomment-1240030015).

### Caller-specified customization

In addition to Go types being able to specify their own JSON representation, the caller of the marshal or unmarshal functionality can also specify their own JSON representation for specific Go types ([#5901](https://github.com/golang/go/issues/5901)). Caller-specified customization takes precedence over type-specified customization.

```go
// SkipFunc may be returned by MarshalToFunc and UnmarshalFromFunc functions.
// Any function that returns SkipFunc must not cause observable side effects
// on the provided Encoder or Decoder.
const SkipFunc = jsonError("skip function")

// Marshalers holds a list of functions that may override the marshal behavior
// of specific types. Populate WithMarshalers to use it.
// A nil *Marshalers is equivalent to an empty list.
type Marshalers struct { /* no exported fields */ }

// JoinMarshalers constructs a flattened list of marshal functions.
// If multiple functions in the list are applicable for a value of a given type,
// then those earlier in the list take precedence over those that come later.
// If a function returns SkipFunc, then the next applicable function is called,
// otherwise the default marshaling behavior is used.
//
// For example:
//
//	m1 := JoinMarshalers(f1, f2)
//	m2 := JoinMarshalers(f0, m1, f3)     // equivalent to m3
//	m3 := JoinMarshalers(f0, f1, f2, f3) // equivalent to m2
func JoinMarshalers(ms ...*Marshalers) *Marshalers

// MarshalFunc constructs a type-specific marshaler that
// specifies how to marshal values of type T.
func MarshalFunc[T any](fn func(T) ([]byte, error)) *Marshalers

// MarshalToFunc constructs a type-specific marshaler that
// specifies how to marshal values of type T.
// The function is always provided with a non-nil pointer value
// if T is an interface or pointer type.
func MarshalToFunc[T any](fn func(*jsontext.Encoder, T, Options) error) *Marshalers

// Unmarshalers holds a list of functions that may override the unmarshal behavior
// of specific types. Populate WithUnmarshalers to use it.
// A nil *Unmarshalers is equivalent to an empty list.
type Unmarshalers struct { /* no exported fields */ }

// JoinUnmarshalers constructs a flattened list of unmarshal functions.
// It operates in a similar manner as [JoinMarshalers].
func JoinUnmarshalers(us ...*Unmarshalers) *Unmarshalers

// UnmarshalFunc constructs a type-specific unmarshaler that
// specifies how to unmarshal values of type T.
func UnmarshalFunc[T any](fn func([]byte, T) error) *Unmarshalers

// UnmarshalFromFunc constructs a type-specific unmarshaler that
// specifies how to unmarshal values of type T.
// T must be an unnamed pointer or an interface type.
// The function is always provided with a non-nil pointer value.
func UnmarshalFromFunc[T any](fn func(*jsontext.Decoder, T, Options) error) *Unmarshalers
```

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

// DefaultOptionsV2 is the full set of all options that define v2 semantics.
// It is equivalent to all options under [Options], [encoding/json.Options],
// and [encoding/json/jsontext.Options] being set to false or the zero value,
// except for the options related to whitespace formatting.
func DefaultOptionsV2() Options

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

// OmitZeroStructFields specifies that a Go struct should marshal in such a way
// that all struct fields that are zero are omitted from the marshaled output
// if the value is zero as determined by the "IsZero() bool" method if present,
// otherwise based on whether the field is the zero Go value.
func OmitZeroStructFields(v bool) Options // affects marshal only

// NonFatalSemanticErrors specifies that [SemanticErrors] encountered
// while marshaling or unmarshaling should not immediately terminate
// the procedure, but that processing should continue and that all
// errors be returned as a multi-error.
func NonFatalSemanticErrors(v bool) Options // affects marshal and unmarshal

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

The `MarshalJSONTo`, `UnmarshalJSONFrom`, `MarshalToFunc`, and `UnmarshalFromFunc` methods and functions take in a singular `Options` value instead of a variadic list because the `Options` type can represent a set of options. The caller (which is the "json" package) can coalesce a list of options before calling the user-specified method or function. Being given a single `Options` value is more ergonomic for the user as there is only one options value to introspect with `GetOption`.

### Errors

Errors due to the inability to correlate JSON data with Go data are reported as `SemanticError`. 
```go
type SemanticError struct {
	// ByteOffset indicates that an error occurred after this byte offset.
	ByteOffset int64
	// JSONPointer indicates that an error occurred within this JSON value
	// as indicated using the JSON Pointer notation (see RFC 6901).
	JSONPointer jsontext.Pointer

	// JSONKind is the JSON kind that could not be handled.
	JSONKind Kind // may be zero if unknown
	// JSONValue is the JSON number or string that could not be unmarshaled.
	JSONValue jsontext.Value // may be nil if irrelevant or unknown
	// GoType is the Go type that could not be handled.
	GoType reflect.Type // may be nil if unknown

	// Err is the underlying error.
	Err error // may be nil
}
func (e *SemanticError) Error() string
func (e *SemanticError) Unwrap() error
```

```go
// ErrUnknownName indicates that a JSON object member could not be
// unmarshaled because the name is not known to the target Go struct.
// This error is directly wrapped within a [SemanticError] when produced.
var ErrUnknownName = errors.New("unknown object member name")
```
`ErrUnknownName` is a sentinel error that is returned
while being wrapped within a `SemanticError`.

## Package "encoding/json"

The API and behavior for v1 "json" remains unchanged except for the addition
of new options to configure v2 to operate with legacy v1 behavior.

### Options

Options may be specified that configures v2 "json" to operate with legacy v1 behavior:
```go
type Options = jsonopts.Options

// DefaultOptionsV1 is the full set of all options that define v1 semantics.
// It is equivalent to the following boolean options being set to true:
//   - [StringifyWithLegacySemantics]
//   - [UnmarshalArrayFromAnyLength]
//
//   - [CallMethodsWithLegacySemantics]
//   - [EscapeInvalidUTF8]
//   - [FormatBytesWithLegacySemantics]
//   - [FormatTimeWithLegacySemantics]
//   - [MatchCaseSensitiveDelimiter]
//   - [MergeWithLegacySemantics]
//   - [OmitEmptyWithLegacyDefinition]
//   - [ReportErrorsWithLegacySemantics]
//   - [jsonv2.Deterministic]
//   - [jsonv2.FormatNilMapAsNull]
//   - [jsonv2.FormatNilSliceAsNull]
//   - [jsonv2.MatchCaseInsensitiveNames]
//   - [jsontext.AllowDuplicateNames]
//   - [jsontext.AllowInvalidUTF8]
//   - [jsontext.EscapeForHTML]
//   - [jsontext.EscapeForJS]
//   - [jsontext.PreserveRawString]
//
// The [Marshal] and [Unmarshal] functions in this package are
// semantically identical to calling the v2 equivalents with this option:
//
//	jsonv2.Marshal(v, jsonv1.DefaultOptionsV1())
//	jsonv2.Unmarshal(b, v, jsonv1.DefaultOptionsV1())
func DefaultOptionsV1() jsonopts.Options

// CallMethodsWithLegacySemantics specifies that calling of type-provided
// marshal and unmarshal methods follow legacy semantics:
//
//   - When marshaling, a marshal method declared on a pointer receiver
//     is only called if the Go value is addressable.
//     Values obtained from an interface or map element are not addressable.
//     Values obtained from a pointer or slice element are addressable.
//     Values obtained from an array element or struct field inherit
//     the addressability of the parent. In contrast, the v2 semantic
//     is to always call marshal methods regardless of addressability.
//
//   - When marshaling or unmarshaling, the [Marshaler] or [Unmarshaler]
//     methods are ignored for map keys. However, [encoding.TextMarshaler]
//     or [encoding.TextUnmarshaler] are still callable.
//     In contrast, the v2 semantic is to serialize map keys
//     like any other value (with regard to calling methods),
//     which may include calling [Marshaler] or [Unmarshaler] methods,
//     where it is the implementation's responsibility to represent the
//     Go value as a JSON string (as required for JSON object names).
//
//   - When marshaling, if a map key value implements a marshal method
//     and is a nil pointer, then it is serialized as an empty JSON string.
//     In contrast, the v2 semantic is to report an error.
//
//   - When marshaling, if an interface type implements a marshal method
//     and the interface value is a nil pointer to a concrete type,
//     then the marshal method is always called.
//     In contrast, the v2 semantic is to never directly call methods
//     on interface values and to instead defer evaluation based upon
//     the underlying concrete value. Similar to non-interface values,
//     marshal methods are not called on nil pointers and
//     are instead serialized as a JSON null.
//
// This affects either marshaling or unmarshaling.
func CallMethodsWithLegacySemantics(bool) jsonopts.Options // affects marshal and unmarshal

// EscapeInvalidUTF8 specifies that when encoding a [jsontext.String]
// with bytes of invalid UTF-8, such bytes are escaped as
// a hexadecimal Unicode codepoint (i.e., \ufffd).
// In contrast, the v2 default is to use the minimal representation,
// which is to encode invalid UTF-8 as the Unicode replacement rune itself
// (without any form of escaping).
func EscapeInvalidUTF8(bool) jsonopts.Options // affects encoding only

// FormatBytesWithLegacySemantics specifies that handling of
// []~byte and [N]~byte types follow legacy semantics:
//
//   - A Go [N]~byte is always treated as as a normal Go array
//     in contrast to the v2 default of treating [N]byte as
//     using some form of binary data encoding (RFC 4648).
//
//   - A Go []~byte is to be treated as using some form of
//     binary data encoding (RFC 4648) in contrast to the v2 default
//     of only treating []byte as such. In particular, v2 does not
//     treat slices of named byte types as representing binary data.
//
//   - When marshaling, if a named byte implements a marshal method,
//     then the slice is serialized as a JSON array of elements,
//     each of which call the marshal method.
//
//   - When unmarshaling, if the input is a JSON array,
//     then unmarshal into the []~byte as if it were a normal Go slice.
//     In contrast, the v2 default is to report an error unmarshaling
//     a JSON array when expecting some form of binary data encoding.
//
//   - When unmarshaling, '\r' and '\n' characters are ignored
//     within the encoded "base32" and "base64" data.
//     In contrast, the v2 default is to report an error in order to be
//     strictly compliant with RFC 4648, section 3.3,
//     which specifies that non-alphabet characters must be rejected.
func FormatBytesWithLegacySemantics(bool) jsonopts.Options // affects marshal and unmarshal

// FormatTimeWithLegacySemantics specifies that [time] types are formatted
// with legacy semantics:
//
//   - When marshaling or unmarshaling, a [time.Duration] is formatted as
//     a JSON number representing the number of nanoseconds.
//     In contrast, the default v2 behavior uses a JSON string
//     with the duration formatted with [time.Duration.String].
//     If a duration field has a `format` tag option,
//     then the specified formatting takes precedence.
//
//   - When unmarshaling, a [time.Time] follows loose adherence to RFC 3339.
//     In particular, it permits historically incorrect representations,
//     allowing for deviations in hour format, sub-second separator,
//     and timezone representation. In contrast, the default v2 behavior
//     is to strictly comply with the grammar specified in RFC 3339.
func FormatTimeWithLegacySemantics(bool) jsonopts.Options // affects marshal and unmarshal

// MatchCaseSensitiveDelimiter specifies that underscores and dashes are
// not to be ignored when performing case-insensitive name matching which
// occurs under [jsonv2.MatchCaseInsensitiveNames] or the `nocase` tag option.
// Thus, case-insensitive name matching is identical to [strings.EqualFold].
// Use of this option diminishes the ability of case-insensitive matching
// to be able to match common case variants (e.g, "foo_bar" with "fooBar").
func MatchCaseSensitiveDelimiter(bool) jsonopts.Options // affects marshal and unmarshal

// MergeWithLegacySemantics specifies that unmarshaling into a non-zero
// Go value follows legacy semantics:
//
//   - When unmarshaling a JSON null, this preserves the original Go value
//     if the kind is a bool, int, uint, float, string, array, or struct.
//     Otherwise, it zeros the Go value.
//     In contrast, the default v2 behavior is to consistently and always
//     zero the Go value when unmarshaling a JSON null into it.
//
//   - When unmarshaling a JSON value other than null, this merges into
//     the original Go value for array elements, slice elements,
//     struct fields (but not map values),
//     pointer values, and interface values (only if a non-nil pointer).
//     In contrast, the default v2 behavior is to merge into the Go value
//     for struct fields, map values, pointer values, and interface values.
//     In general, the v2 semantic merges when unmarshaling a JSON object,
//     otherwise it replaces the original value.
func MergeWithLegacySemantics(bool) jsonopts.Options // affects unmarshal only

// OmitEmptyWithLegacyDefinition specifies that the `omitempty` tag option
// follows a definition of empty where a field is omitted if the Go value is
// false, 0, a nil pointer, a nil interface value,
// or any empty array, slice, map, or string.
// This overrides the v2 semantic where a field is empty if the value
// marshals as a JSON null or an empty JSON string, object, or array.
//
// The v1 and v2 definitions of `omitempty` are practically the same for
// Go strings, slices, arrays, and maps. Usages of `omitempty` on
// Go bools, ints, uints floats, pointers, and interfaces should migrate to use
// the `omitzero` tag option, which omits a field if it is the zero Go value.
func OmitEmptyWithLegacyDefinition(bool) jsonopts.Options // affects marshal only

// ReportErrorsWithLegacySemantics specifies that Marshal and Unmarshal
// should report errors with legacy semantics:
//
//   - When marshaling or unmarshaling, the returned error values are
//     usually of types such as [SyntaxError], [MarshalerError],
//     [UnsupportedTypeError], [UnsupportedValueError],
//     [InvalidUnmarshalError], or [UnmarshalTypeError].
//     In contrast, the v2 semantic is to always return errors as either
//     [jsonv2.SemanticError] or [jsontext.SyntacticError].
//
//   - When marshaling, if a user-defined marshal method reports an error,
//     it is always wrapped in a [MarshalerError], even if the error itself
//     is already a [MarshalerError], which may lead to multiple redundant
//     layers of wrapping. In contrast, the v2 semantic is to
//     always wrap an error within [jsonv2.SemanticError]
//     unless it is already a semantic error.
//
//   - When unmarshaling, if a user-defined unmarshal method reports an error,
//     it is never wrapped and reported verbatim. In contrast, the v2 semantic
//     is to always wrap an error within [jsonv2.SemanticError]
//     unless it is already a semantic error.
//
//   - When marshaling or unmarshaling, if a Go struct contains type errors
//     (e.g., conflicting names or malformed field tags), then such errors
//     are ignored and the Go struct uses a best-effort representation.
//     In contrast, the v2 semantic is to report a runtime error.
//
//   - When unmarshaling, the syntactic structure of the JSON input
//     is fully validated before performing the semantic unmarshaling
//     of the JSON data into the Go value. Practically speaking,
//     this means that JSON input with syntactic errors do not result
//     in any mutations of the target Go value. In contrast, the v2 semantic
//     is to perform a streaming decode and gradually unmarshal the JSON input
//     into the target Go value, which means that the Go value may be
// StringifyWithLegacySemantics specifies that the `string` tag option
// may stringify bools and string values. It only takes effect on fields
//     partially mutated when a syntactic error is encountered.
//
//   - When unmarshaling, a semantic error does not immediately terminate the
//     unmarshal procedure, but rather evaluation continues.
//     When unmarshal returns, only the first semantic error is reported.
//     In contrast, the v2 semantic is to terminate unmarshal the moment
//     an error is encountered.
func ReportErrorsWithLegacySemantics(bool) jsonopts.Options // affects marshal and unmarshal

// where the top-level type is a bool, string, numeric kind, or a pointer to
// such a kind. Specifically, `string` will not stringify bool, string,
func StringifyWithLegacySemantics(bool) jsonopts.Options // affects marshal only
// or numeric kinds within a composite data type
// (e.g., array, slice, struct, map, or interface).
//
// When marshaling, such Go values are serialized as their usual
// JSON representation, but quoted within a JSON string.
// When unmarshaling, such Go values must be deserialized from
// a JSON string containing their usual JSON representation.
// A JSON null quoted in a JSON string is a valid substitute for JSON null
// while unmarshaling into a Go value that `string` takes effect on.

// UnmarshalArrayFromAnyLength specifies that Go arrays can be unmarshaled
// from input JSON arrays of any length. If the JSON array is too short,
// then the remaining Go array elements are zeroed. If the JSON array
// is too long, then the excess JSON array elements are skipped over.
func UnmarshalArrayFromAnyLength(bool) jsonopts.Options // affects unmarshal only
```

Many of the options configure fairly obscure behavior.
Unfortunately, many of the behaviors cannot be changed due in order to maintain
backwards compatibility. This is a major justification for a v2 "json" package.

### Types aliases

The following types are moved to v2 "json":
```go
type Marshaler = jsonv2.Marshaler
type Unmarshaler = jsonv2.Unmarshaler
type RawMessage = jsontext.Value
```

### Number methods

The `Number` type no longer has special-case support in the "json" implementation itself.
```go
func (Number) MarshalJSONTo(*jsontext.Encoder, jsonopts.Options) error
func (*Number) UnmarshalJSONFrom(*jsontext.Decoder, jsonopts.Options) error
```
So methods are added to have it implement the v2 `MarshalerTo` and `UnmarshalerFrom` methods
to preserve equivalent behavior.

### Errors

The `UnmarshalTypeError` type is extended to wrap an underlying error:
```go
type UnmarshalTypeError struct {
	...
	Err error
}

func (*UnmarshalTypeError) Unwrap() error
```
Errors returned v2 "json" are much richer, so the wrapped error provides
a way for v1 "json" to preserve some of that context,
while still using the `UnmarshalTypeError` type,
which many programs may still be expecting.

The `UnmarshalTypeError.Field` now reports a dot-delimited path to the error value
where each path segment is either a JSON array and map index operation.
This is a divergence from prior behavior which was always inconsistent
about whether the position was reported according to the Go namespace
or the JSON namespace (see #43126).

## Proposed implementation

This proposal has been implemented by the [`github.com/go-json-experiment/json`](https://pkg.go.dev/github.com/go-json-experiment/json) module.

If this proposal is accepted, the implementation in `github.com/go-json-experiment/json`
will be moved into the standard library.

We may also provide a `golang.org/x/json` module that contains an identical copy
of the implementation so that users on older Go releases can make use of v2.
This module will use type-aliases to the Go standard library if the user
is compiling with a sufficiently new version of the Go toolchain.

## Changes from discussion

If you have already read the discussion in #63397,
then much of the API presented above may be familiar.
This section records the differences made relative to the discussion.

### Package "encoding/json/jsontext"

The following `Value` methods were altered to accept options.
```diff
- func (v Value) IsValid() bool
+ func (v Value) IsValid(opts ...Options) bool
- func (v *Value) Compact() error
+ func (v *Value) Compact(opts ...Options) error
- func (v *Value) Indent(prefix, indent string) error
+ func (v *Value) Indent(opts ...Options) error
- func (v *Value) Canonicalize() error
+ func (v *Value) Canonicalize(opts ...Options) error

+ func (v *Value) Format(opts ...Options) error
```
Accepting options allows the default behavior of these methods to be overridden,
providing greater flexibility in usage.

The removal of the `prefix` and `indent` argument from `Indent` improves
the ergonomics of the method as most users just want indented output
without thinking about the particular `indent` string used.
These can still be specified using the `WithIndentPrefix` and `WithIndent` options.

One major criticism of `Canonicalize` (per RFC 8785) is that it mangles
the precision of wide integers. By accepting options, users can
additionally specify `CanonicalizeRawInts(false)` to prevent this behavior,
while still having canonicalization for all other JSON artifacts.

The `Format` method was newly added as the primary implementation backing
`Compact`, `Indent`, and `Canonicalize`.

The following options were added to provide greater flexibility to formatting:
```diff
+ func CanonicalizeRawFloats(v bool) Options
+ func CanonicalizeRawInts(v bool) Options
+ func PreserveRawStrings(v bool) Options
+ func ReorderRawObjects(v bool) Options
+ func SpaceAfterComma(v bool) Options
+ func SpaceAfterColon(v bool) Options

- func Expand(v bool) Options
+ func Multiline(v bool) Options
```
The `Expand` option was renamed as `Multiline` to be more clear and
to distinguish it from `SpaceAfterComma` and `SpaceAfterColon`
(which both technically "expand" the output).

The following formatting API has been added:
```diff
+ func AppendFormat(dst, src []byte, opts ...Options) ([]byte, error)
+ func AppendQuote[Bytes ~[]byte | ~string](dst []byte, src Bytes) ([]byte, error)
+ func AppendUnquote[Bytes ~[]byte | ~string](dst []byte, src Bytes) ([]byte, error)
```

The length returned by `StackIndex` is now a `int64` instead of `int`
since the length of a JSON array or object could theoretically exceed `int`
when handling JSON in a purely streaming manner.
```diff
-func (e *Encoder) StackIndex(i int) (Kind, int)
+func (e *Encoder) StackIndex(i int) (Kind, int64)
-func (d *Decoder) StackIndex(i int) (Kind, int)
+func (d *Decoder) StackIndex(i int) (Kind, int64)
```

The pointer returned by `StackPointer` is now the named `Pointer` type.
```diff
- func (e *Encoder) StackPointer() string
+ func (e *Encoder) StackPointer() Pointer
- func (d *Decoder) StackPointer() string
+ func (d *Decoder) StackPointer() Pointer
```

Handling of errors was improved:
```diff
  type SyntacticError struct {
  	...
- 	JSONPointer string
+ 	JSONPointer Pointer
  }

+ var ErrDuplicateName = errors.New("duplicate object member name")
+ var ErrNonStringName = errors.New("object member name must be a string")

+ type Pointer string
+ func (p Pointer) IsValid() bool
+ func (p Pointer) AppendToken(tok string) Pointer
+ func (p Pointer) Parent() Pointer
+ func (p1 Pointer) Contains(p2 Pointer) bool
+ func (p Pointer) LastToken() string
+ func (p Pointer) Tokens() iter.Seq[string]
```
An explicit `Pointer` type was added to represent a JSON Pointer (RFC 6901)
as a means to identify exactly where an error occurred.

Convenience methods are defined for interacting with a pointer.

The `ErrDuplicateName` and `ErrNonStringName` errors were added to support
common error conditions users may want to distinguish uppon
through the use of `errors.Is`.

### Package "encoding/json/v2"

Interface types and methods were renamed to avoid the `V1` and `V2` suffixes,
which were aesthetically unpleasant. Instead, the `V2` declarations now
generally use the `To` and `From` suffixes to indicate that they support streaming.
This follows after the convention established by `io.WriterTo` and `io.ReaderFrom`.
```diff
- type MarshalerV1 interface { MarshalJSON() ([]byte, error) }
+ type Marshaler interface { MarshalJSON() ([]byte, error)}

- type MarshalerV2 interface { MarshalJSONV2(*jsontext.Encoder, Options) error }
+ type MarshalerTo interface { MarshalJSONTo(*jsontext.Encoder, Options) error}

- type UnmarshalerV1 interface { UnmarshalJSON([]byte) error }
+ type Unmarshaler interface { UnmarshalJSON([]byte) error}

- type UnmarshalerV2 interface { UnmarshalJSONV2(*jsontext.Decoder, Options) error }
+ type UnmarshalerFrom interface { UnmarshalJSONFrom(*jsontext.Decoder, Options) error}

- func MarshalFuncV1[T any](fn func(T) ([]byte, error)) *Marshalers
+ func MarshalFunc[T any](fn func(T) ([]byte, error)) *Marshalers

- func MarshalFuncV2[T any](fn func(*jsontext.Encoder, T, Options) error) *Marshalers
+ func MarshalToFunc[T any](fn func(*jsontext.Encoder, T, Options) error) *Marshalers

- func UnmarshalFuncV1[T any](fn func([]byte, T) error) *Unmarshalers
+ func UnmarshalFunc[T any](fn func([]byte, T) error) *Unmarshalers

- func UnmarshalFuncV2[T any](fn func(*jsontext.Decoder, T, Options) error) *Unmarshalers
+ func UnmarshalFromFunc[T any](fn func(*jsontext.Decoder, T, Options) error) *Unmarshalers
```

The constructor for `Marshalers` and `Unmarshalers` were renamed using the `Join` prefix
to be more consistent with the existing `JoinOptions` constructor.
It also more clearly matches exactly what the constructor does.
```diff
- func NewMarshalers(ms ...*Marshalers) *Marshalers
+ func JoinMarshalers(ms ...*Marshalers) *Marshalers

- func NewUnmarshalers(us ...*Unmarshalers) *Unmarshalers
+ func JoinUnmarshalers(us ...*Unmarshalers) *Unmarshalers
```

The following options were added:
```diff
+ func OmitZeroStructFields(v bool) Options
+ func NonFatalSemanticErrors(v bool) Options
```
The `OmitZeroStructFields` is a caller-specified option that mirrors
the addition of the `omitzero` struct tag option.

Implementing v1 in terms of v2 required the latter to support non-fatal errors.
The `NonFatalSemanticErrors` option exposes that functionality in a
more consistent (i.e., handling both marshal and unmarshal) and
in a more modern way (i.e., returning a multi error).

```diff
  type SemanticError struct {
  	...
- 	JSONPointer string
+ 	JSONPointer jsontext.Pointer
+ 	JSONValue   jsontext.Value
  }

+ var ErrUnknownName = errors.New("unknown object member name")
```

The `ErrUnknownName` error was added to support common use-cases
wanting to distinguish this particular condition (see #29035).

The following behavior changes were made to marshal and unmarshal:

* There is newely added support for the `strictcase` option to provide
  a better migration path between users of both v1 and v2.

* Specificying the `string` tag option now rejects also unmarshaling
  from a JSON number and only permits unmarshaling from a JSON string.
  This exactly matches the behavior of v1.

* When unmarshaling, a floating-point overflow results in an error.
  This exactly matches the behavior of v1.

* Serialization now supports embeddedd fields of unexported struct types
  with exported fields.
  This exactly matches the behavior of v1.

### Package "encoding/json"

Some options for legacy v1 support were renamed or
had similar options folded together.
```diff
- func RejectFloatOverflow(v bool) Options

- func IgnoreStructErrors(v bool) Options
- func ReportLegacyErrorValues(v bool) Options
+ func ReportErrorsWithLegacySemantics(bool)

- func SkipUnaddressableMethods(v bool) Options
+ func CallMethodsWithLegacySemantics(bool)

- func FormatByteArrayAsArray(v bool) Options
+ func FormatBytesWithLegacySemantics(bool)

- func FormatTimeDurationAsNanosecond(v bool) Options
+ func FormatTimeWithLegacySemantics(bool)

+ func EscapeInvalidUTF8(bool)
```
In general, many options were renamed with a `WithLegacySemantics` suffix
because they convered a multitude of behavior differences that could
not be adequently described with a concise name.

The `RejectFloatOverflow` option was removed because v2 now rejects
floating-point overflows just like v1.

The `EscapeInvalidUTF8` option was added in order to support a
behavior difference that was discovered while implementing
v1 support in terms of v2.

The `Number` type implements `MarshalerTo` and `UnmarshalerFrom` for
better compatibility with v2.
```diff
+ func (Number) MarshalJSONTo(*jsontext.Encoder, jsonopts.Options) error
+ func (*Number) UnmarshalJSONFrom(*jsontext.Decoder, jsonopts.Options) error
```

The `UnmarshalTypeError` type now supports error wrapping.
```diff
  type UnmarshalTypeError struct {
  	...
+ 	Err error
  }

+ func (*UnmarshalTypeError) Unwrap() error
```
