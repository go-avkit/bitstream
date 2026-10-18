// Copyright (c) 2026, go-avkit
// SPDX-License-Identifier: BSD-3-Clause

package bitstream

import (
	"errors"
	"testing"
)

// ⛔ A length of 2^31 or more is an int below zero where int is 32 bits wide,
// and SplitLengthPrefixed converted before it compared: the negative length
// passed the overrun check and sliced backwards, a panic on arm, 386 and every
// other 32-bit target. On a 64-bit target the conversion is exact and this
// test passes either way; the failure it guards was reproduced under
// GOARCH=arm (linux/arm/v7), where it panicked with
// "slice bounds out of range [:-201323878]" on a real H.264 stream.
func TestSplitLengthPrefixedHugeLengthIsAnOverrun(t *testing.T) {
	for _, length := range [][]byte{
		{0xFF, 0xFF, 0xFF, 0xFF},
		{0x80, 0x00, 0x00, 0x00},
		{0xFF, 0xFF, 0xFF, 0xFB}, // -5 as an int32: lands back on the length itself
	} {
		data := append(append([]byte(nil), length...), 1, 2, 3, 4, 5, 6, 7, 8)
		if _, err := SplitLengthPrefixed(data, 4); !errors.Is(err, ErrLengthOverrun) {
			t.Errorf("length % x: err = %v, want ErrLengthOverrun", length, err)
		}
	}
}
