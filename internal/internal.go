// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

// NotForPublicUse is a marker type that an API is for internal use only.
// It does not perfectly prevent usage of that API, but helps to restrict usage.
// Anything with this marker is not covered by the Go compatibility agreement.
type NotForPublicUse struct{}

// AllowInternalUse is passed from "json" to "jsontext" to authenticate
// that the caller can have access to internal functionality.
var AllowInternalUse NotForPublicUse

var (
	// TransformMarshalError converts a v2 error into a v1 error.
	TransformMarshalError func(error) error
	// TransformUnmarshalError converts a v2 error into a v1 error.
	TransformUnmarshalError func(error) error
	// NewRawNumber returns new(jsonv1.Number).
	NewRawNumber func() any
	// RawNumberOf returns jsonv1.Number(b).
	RawNumberOf func(b []byte) any
)
