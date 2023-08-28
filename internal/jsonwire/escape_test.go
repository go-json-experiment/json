// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonwire

import (
	"reflect"
	"testing"
)

func TestEscapeRunesTables(t *testing.T) {
	tests := []struct {
		got  *EscapeRunes
		want *EscapeRunes
	}{
		{&escapeCanonical, makeEscapeRunesSlow(false, false, nil)},
		{&escapeHTMLJS, makeEscapeRunesSlow(true, true, nil)},
		{&escapeHTML, makeEscapeRunesSlow(true, false, nil)},
		{&escapeJS, makeEscapeRunesSlow(false, true, nil)},
	}
	for _, tt := range tests {
		if !reflect.DeepEqual(tt.got, tt.want) {
			t.Errorf("table mismatch:\n\tgot:  %+v\n\twant: %+v", tt.got, tt.want)
		}
	}
}
