// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bufpools

import (
	"bytes"
	"crypto/md5"
	"io"
	"math/bits"
	"math/rand"
	"testing"
)

func FuzzBuffer(f *testing.F) {
	f.Add(int64(0))
	f.Fuzz(func(t *testing.T, seed int64) {
		const maxCapacity = 1 << 20
		rn := rand.New(rand.NewSource(seed))
		var gotBuffer Buffer
		var wantBuffer bytes.Buffer
		var exit bool
		for i := 0; !exit; i++ {
			exit = gotBuffer.Cap() > maxCapacity || wantBuffer.Cap() > maxCapacity || i >= 100
			switch {
			case gotBuffer.Len() != wantBuffer.Len():
				t.Fatalf("Buffer.Len = %d, want %d", gotBuffer.Len(), wantBuffer.Len())
			case gotBuffer.Len() > gotBuffer.Cap():
				t.Fatalf("Buffer.Len > Buffer.Cap: %d > %d", gotBuffer.Len(), gotBuffer.Cap())
			case len(gotBuffer.AvailableBuffer()) > 0:
				t.Fatalf("len(Buffer.AvailableBuffer) = %d, want 0", len(gotBuffer.AvailableBuffer()))
			case cap(gotBuffer.AvailableBuffer()) > gotBuffer.Cap()-gotBuffer.Len():
				t.Fatalf("cap(Buffer.AvailableBuffer) = %d, want %d", cap(gotBuffer.AvailableBuffer()), gotBuffer.Cap()-gotBuffer.Len())
			}

			// Random size that is exponentially spaced.
			n := (1 << rn.Intn(bits.Len(maxCapacity-1))) >> 1
			n += rn.Intn(n + 1)

			// Randomly perform some mutating buffer operation.
			switch j := rn.Intn(20); {
			case j < 8 && !exit: // 40% probability
				b := gotBuffer.AvailableBuffer()
				b = b[:rn.Intn(cap(b)+1)]
				io.ReadFull(rn, b)
				gotBuffer.Write(b)
				wantBuffer.Write(b)
			case j < 14 && !exit: // 30% probability
				b := make([]byte, n)
				io.ReadFull(rn, b)
				gotBuffer.Write(b)
				wantBuffer.Write(b)
			case j < 19 && !exit: // 25% probability
				gotBuffer.Grow(n)
			default: // 5% probability
				var gotBytes []byte
				if rn.Intn(2) == 0 {
					gotBytes = gotBuffer.Bytes()
				} else {
					gotBytes = gotBuffer.BytesClone()
				}
				if !bytes.Equal(gotBytes, wantBuffer.Bytes()) {
					t.Fatalf("content mismatch: %x != %x", md5.Sum(gotBytes), md5.Sum(wantBuffer.Bytes()))
				}
				gotBuffer.Reset()
				wantBuffer.Reset()
				if gotBuffer.Len() > 0 {
					t.Fatalf("Buffer.Len = %d, want 0", gotBuffer.Len())
				}
			}
		}
	})
}
