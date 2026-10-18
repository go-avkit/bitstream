// Copyright (c) 2026, go-avkit
// SPDX-License-Identifier: BSD-3-Clause

package bitstream

import (
	"bytes"
	"os"
	"testing"
)

// The fuzz targets below hold this package to what a reader of downloaded
// streams needs from it: no input, however malformed, panics or walks the
// reader outside its data, and every unit handed back is a slice of the input.
// An error is always an acceptable answer.
//
//	go test -run='^$' -fuzz='^FuzzSplitAnnexB$' -fuzztime=60s

// streamSeeds adds the seed streams: a real H.264 and a real H.265 elementary
// stream (testdata/fuzzseed), and a few hand-made edge cases.
func streamSeeds(f *testing.F) [][]byte {
	f.Helper()
	var seeds [][]byte
	for _, name := range []string{"testdata/fuzzseed/h264.264", "testdata/fuzzseed/hevc.265"} {
		data, err := os.ReadFile(name)
		if err != nil {
			f.Fatalf("seed: %v", err)
		}
		seeds = append(seeds, data)
	}
	return append(seeds,
		nil,
		[]byte{0, 0, 1},
		[]byte{0, 0, 0, 1, 0x67, 0, 0, 3, 0, 0, 0, 1},
		[]byte{0xFF, 0xFF, 0xFF, 0xFF, 1, 2, 3},
		[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
}

// within says whether unit is a slice of data's own memory.
func within(data, unit []byte) bool {
	if len(unit) == 0 || len(data) == 0 {
		return false
	}
	for i := range data {
		if &data[i] == &unit[0] {
			return i+len(unit) <= len(data)
		}
	}
	return false
}

func FuzzSplitAnnexB(f *testing.F) {
	for _, s := range streamSeeds(f) {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		units, err := SplitAnnexB(data)
		if err != nil {
			return
		}
		for i, u := range units {
			if !within(data, u) {
				t.Fatalf("unit %d is not a slice of the input", i)
			}
			if u[len(u)-1] == 0 {
				t.Fatalf("unit %d keeps a trailing zero", i)
			}
		}
	})
}

func FuzzSplitLengthPrefixed(f *testing.F) {
	for _, s := range streamSeeds(f) {
		for _, size := range []uint8{1, 2, 4} {
			f.Add(s, size)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte, size uint8) {
		// Six values, so the refused widths 0 and 5 are reached as well.
		units, err := SplitLengthPrefixed(data, int(size%6))
		if err != nil {
			return
		}
		total := 0
		for i, u := range units {
			if !within(data, u) {
				t.Fatalf("unit %d is not a slice of the input", i)
			}
			total += len(u)
		}
		if total > len(data) {
			t.Fatalf("%d bytes of units from %d bytes of input", total, len(data))
		}
	})
}

func FuzzUnescape(f *testing.F) {
	for _, s := range streamSeeds(f) {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		out := Unescape(data)
		if len(out) > len(data) {
			t.Fatalf("unescaping grew %d bytes to %d", len(data), len(out))
		}
		// Only a three is ever dropped.
		if bytes.Count(data, []byte{3})-bytes.Count(out, []byte{3}) != len(data)-len(out) {
			t.Fatalf("a byte other than an escape was dropped")
		}
	})
}

// FuzzReader drives a Reader through a sequence of reads chosen by the first
// half of the input, over the second half.
func FuzzReader(f *testing.F) {
	for _, s := range streamSeeds(f) {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		ops, payload := data[:len(data)/2], data[len(data)/2:]
		r := NewReader(payload)
		for _, op := range ops {
			switch op % 7 {
			case 0:
				_, _ = r.Bit()
			case 1:
				// Widths past both ends of what Bits accepts.
				_, _ = r.Bits(int(op>>3) - 4)
			case 2:
				_, _ = r.UE()
			case 3:
				_, _ = r.SE()
			case 4:
				at := r.Pos()
				_, _ = r.Peek(int(op >> 3))
				if r.Pos() != at {
					t.Fatalf("Peek moved the reader from %d to %d", at, r.Pos())
				}
			case 5:
				_ = r.MoreData()
			case 6:
				_ = r.Left()
			}
			if r.Pos() < 0 || r.Pos() > len(payload)*8 || r.Left() != len(payload)*8-r.Pos() {
				t.Fatalf("position %d, %d left, over %d bytes", r.Pos(), r.Left(), len(payload))
			}
		}
	})
}
