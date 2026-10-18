// Copyright (c) 2026, go-avkit
// SPDX-License-Identifier: BSD-3-Clause

package bitstream

import (
	"errors"
	"strings"
	"testing"
)

// ⛔ An Exp-Golomb code of 32 leading zeros states 2^32 - 1 plus a 32-bit
// suffix. Only a zero suffix fits the uint32 UE returns; any other wrapped
// around, so 32 zeros, a one, then the suffix 1 read as ZERO -- a value nobody
// wrote, accepted without a word, from 65 bits that cannot state anything a
// uint32 holds. It is refused now as the over-long code it is.
func TestExpGolombPastTheTopIsRefusedNotWrapped(t *testing.T) {
	code := fromBits(strings.Repeat("0", 32) + "1" + strings.Repeat("0", 31) + "1")
	if v, err := NewReader(code).UE(); !errors.Is(err, ErrTooLong) {
		t.Errorf("UE = %d, %v; want ErrTooLong", v, err)
	}
	if v, err := NewReader(code).SE(); !errors.Is(err, ErrTooLong) {
		t.Errorf("SE = %d, %v; want ErrTooLong", v, err)
	}
}

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
