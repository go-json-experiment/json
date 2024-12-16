// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"errors"

	"github.com/go-json-experiment/json/internal"
	"github.com/go-json-experiment/json/internal/jsonflags"
	"github.com/go-json-experiment/json/jsontext"
)

// export exposes internal functionality of the "jsontext" package.
var export = jsontext.Internal.Export(&internal.AllowInternalUse)

// Valid reports whether data is a valid JSON encoding.
func Valid(data []byte) bool {
	return checkValid(data) == nil
}

func checkValid(data []byte) error {
	d := export.GetBufferedDecoder(data)
	defer export.PutBufferedDecoder(d)
	xd := export.Decoder(d)
	xd.Struct.Flags.Set(jsonflags.AllowDuplicateNames | jsonflags.AllowInvalidUTF8 | 1)
	if _, err := d.ReadValue(); err != nil {
		return transformSyntacticError(err)
	}
	if err := xd.CheckEOF(); err != nil {
		return transformSyntacticError(err)
	}
	return nil
}

// A SyntaxError is a description of a JSON syntax error.
// [Unmarshal] will return a SyntaxError if the JSON can't be parsed.
type SyntaxError struct {
	msg    string // description of error
	Offset int64  // error occurred after reading Offset bytes
}

func (e *SyntaxError) Error() string { return e.msg }

func transformSyntacticError(err error) error {
	switch serr, ok := err.(*jsontext.SyntacticError); {
	case serr != nil:
		return &SyntaxError{Offset: serr.ByteOffset, msg: serr.Error()}
	case ok:
		return (*SyntaxError)(nil)
	case export.IsIOError(err):
		return errors.Unwrap(err) // v1 historically did not wrap IO errors
	default:
		return err
	}
}
