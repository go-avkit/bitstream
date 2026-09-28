// Copyright (c) 2026, go-avkit
// SPDX-License-Identifier: BSD-3-Clause

package bitstream

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Errors framing can refuse with.
var (
	// ErrNoStartCode means a byte stream holds no unit at all.
	ErrNoStartCode = errors.New("bitstream: no start code in the byte stream")
	// ErrLengthOverrun means a length-prefixed unit claims more bytes than the
	// stream holds.
	ErrLengthOverrun = errors.New("bitstream: unit length runs past the end")
	// ErrEmptySize means a length size is not one these formats use.
	ErrEmptySize = errors.New("bitstream: unit length size is not 1 to 4")
)

// SplitAnnexB returns the units of a byte stream, the form a raw elementary
// stream and an MPEG-TS stream carry.
//
// ⛔ The units are returned as they lie, header bytes included, because how many
// of those bytes are header is the ONE thing that differs between the codecs
// sharing this: H.264 spends one and H.265 spends two. Reading them here would
// put a codec's rule in the one place that must not have one.
//
// Units are separated by two or more zero bytes and a one. Three zeros and a one
// is the same separator with a leading zero, so the scan looks for the three-byte
// form and lets a fourth zero belong to the gap rather than to the unit -- a unit
// that began with a stray zero would have its header read out of a byte nobody
// wrote there. Trailing zeros are dropped for the same reason in reverse: an
// encoder may pad, and padding is not payload.
//
// A stream cut after its last separator keeps the units that did arrive: a cut
// stream is the normal state of a download in progress, and refusing the whole of
// it would lose everything that was there.
func SplitAnnexB(data []byte) ([][]byte, error) {
	starts := startCodes(data)
	if len(starts) == 0 {
		return nil, ErrNoStartCode
	}
	out := make([][]byte, 0, len(starts))
	for i, at := range starts {
		end := len(data)
		if i+1 < len(starts) {
			end = starts[i+1].at
		}
		body := data[at.after:end]
		for len(body) > 0 && body[len(body)-1] == 0 {
			body = body[:len(body)-1]
		}
		if len(body) == 0 {
			continue
		}
		out = append(out, body)
	}
	if len(out) == 0 {
		return nil, ErrNoStartCode
	}
	return out, nil
}

// startCode is where one starts and where the unit after it begins.
type startCode struct{ at, after int }

// startCodes finds every 00 00 01 in data.
func startCodes(data []byte) []startCode {
	var out []startCode
	for i := 0; i+2 < len(data); {
		if data[i] == 0 && data[i+1] == 0 && data[i+2] == 1 {
			out = append(out, startCode{at: i, after: i + 3})
			i += 3
			continue
		}
		i++
	}
	return out
}

// SplitLengthPrefixed returns the units of the form an MP4 sample carries, where
// each is preceded by its length in sizeBytes bytes.
//
// sizeBytes comes from the codec's configuration record and is 1, 2 or 4 in
// practice; anything outside 1 to 4 is refused rather than guessed, since a wrong
// size reads a length out of payload and would walk the stream into nonsense.
func SplitLengthPrefixed(data []byte, sizeBytes int) ([][]byte, error) {
	if sizeBytes < 1 || sizeBytes > 4 {
		return nil, fmt.Errorf("%w: %d", ErrEmptySize, sizeBytes)
	}
	var out [][]byte
	for off := 0; off < len(data); {
		if off+sizeBytes > len(data) {
			return nil, fmt.Errorf("%w: %d bytes left, a length needs %d",
				ErrLengthOverrun, len(data)-off, sizeBytes)
		}
		n := int(readLength(data[off : off+sizeBytes]))
		off += sizeBytes
		if n == 0 {
			// A zero length is padding an encoder may write, and describes no
			// unit at all.
			continue
		}
		if off+n > len(data) {
			return nil, fmt.Errorf("%w: %d bytes claimed, %d left", ErrLengthOverrun, n, len(data)-off)
		}
		out = append(out, data[off:off+n])
		off += n
	}
	return out, nil
}

// readLength reads a big-endian length of one to four bytes.
func readLength(b []byte) uint32 {
	switch len(b) {
	case 1:
		return uint32(b[0])
	case 2:
		return uint32(binary.BigEndian.Uint16(b))
	case 3:
		return uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
	default:
		return binary.BigEndian.Uint32(b)
	}
}

// Unescape undoes the escaping inside a unit.
//
// These formats may not carry three consecutive bytes that look like a start
// code, so an encoder writes 00 00 03 where it means 00 00 and the reader drops
// the three.
//
// ⛔ Only a three that follows exactly two zeros is an escape: dropping every
// three after any zero would eat payload, and the byte after the escape is
// whatever it is -- including another zero, which starts the count again.
func Unescape(unit []byte) []byte {
	out := make([]byte, 0, len(unit))
	zeros := 0
	for _, b := range unit {
		if zeros == 2 && b == 3 {
			zeros = 0
			continue
		}
		if b == 0 {
			zeros++
		} else {
			zeros = 0
		}
		out = append(out, b)
	}
	return out
}
