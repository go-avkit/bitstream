// Copyright (c) 2026, go-avkit
// SPDX-License-Identifier: BSD-3-Clause

// Package bitstream holds what the video codecs of the ITU-T H.26x family read
// their headers with, in pure Go.
//
// It exists because two codecs needed the same code and neither should depend on
// the other. H.264 and H.265 frame their units identically -- the same start
// codes, the same escaping inside a unit, the same variable-length integers -- and
// differ only in how they read the header bytes of a unit once it is found. That
// difference is the seam: this package finds the units and reads the bits, and
// knows nothing about what a unit means.
//
// Measured before it was written: the framing of a real HEVC stream, split by the
// H.264 code this was lifted from, gave 65 units whose types read the H.265 way
// are exactly what such a stream holds -- one video parameter set, one sequence
// set, one picture set, one IDR picture, thirty trailing pictures and their SEI
// messages. The framing transfers; the header does not.
package bitstream
