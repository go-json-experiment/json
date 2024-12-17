// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import "github.com/go-json-experiment/json/internal"

// Inject functionality into v2 to properly handle v1 types.
func init() {
	internal.NewRawNumber = func() any { return new(Number) }
	internal.RawNumberOf = func(b []byte) any { return Number(b) }
}
