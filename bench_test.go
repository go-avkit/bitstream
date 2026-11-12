// Copyright (c) 2026, go-avkit
// SPDX-License-Identifier: BSD-3-Clause

package bitstream

import (
	"math/rand"
	"testing"
)

// BenchmarkBits reads a 4 KiB payload in 13-bit fields.
func BenchmarkBits(b *testing.B) {
	data := make([]byte, 4096)
	rand.New(rand.NewSource(1)).Read(data)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r := NewReader(data)
		for r.Left() >= 13 {
			if _, err := r.Bits(13); err != nil {
				b.Fatal(err)
			}
		}
	}
}
