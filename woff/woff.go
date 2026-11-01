package woff

import (
	"bytes"
	"cmp"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
)

// https://www.w3.org/TR/WOFF/

var ErrBadChecksum = errors.New("bad table checksum")

// Decode decompresses a [WOFF] font, appending the resulting TTF/OTF to dst.
//
// If the checksum is incorrect (note that browsers don't usually validate it),
// [ErrBadChecksum] will be returned in addition to the font being appended to
// dst (i.e., you could check err != nil && err != ErrBadChecksum).
func Decode(dst, src []byte) ([]byte, error) {
	f, err := parseHeader(src)
	if err != nil {
		return dst, err
	}
	out := slices.Grow(dst, f.size)
	err = f.write(out[len(dst):len(dst)+f.size], src)
	if err == nil || err == ErrBadChecksum {
		dst = out[:len(dst)+f.size]
	}
	return dst, err
}

// DecodeLength returns the size of the decoded TTF/OTF font for the given WOFF
// input. It may not always return an error for invalid input.
func DecodeLength(src []byte) (int, error) {
	f, err := parseHeader(src)
	if err != nil {
		return 0, err
	}
	return f.size, nil
}

type font struct {
	flavor []byte
	tables []woffTable // sorted by tag
	order  []int       // indices into tables in WOFF data order
	head   uint32      // sfnt offset of the head table
	size   int
}

type woffTable struct {
	tag          string
	offset       uint32
	length       uint32
	origLength   uint32
	origChecksum uint32
	sfntOffset   uint32
}

func parseHeader(src []byte) (*font, error) {
	if len(src) < 44 {
		return nil, fmt.Errorf("invalid font data")
	}

	be := binary.BigEndian
	if string(src[0:4]) != "wOFF" {
		return nil, fmt.Errorf("bad signature %q", src[0:4])
	}
	flavor := src[4:8]
	length := be.Uint32(src[8:])
	numTables := be.Uint16(src[12:])
	reserved := be.Uint16(src[14:])
	// remaining header fields: totalSfntSize, majorVersion, minorVersion,
	// metaOffset, metaLength, metaOrigLength, privOffset, privLength

	if numTables == 0 {
		return nil, fmt.Errorf("numTables in header is zero")
	} else if numTables >= 4096 {
		// 16*numTables must fit in the uint16 searchRange and rangeShift
		return nil, fmt.Errorf("numTables %d in header is too large", numTables)
	} else if reserved != 0 {
		return nil, fmt.Errorf("reserved in header is not zero (got %d)", reserved)
	}
	frontSize := 44 + 20*uint32(numTables)
	if length <= frontSize {
		return nil, fmt.Errorf("invalid font data")
	} else if uint64(length) != uint64(len(src)) {
		return nil, fmt.Errorf("length %d in header does not match file size %d", length, len(src))
	}

	tables := make([]woffTable, numTables)
	for i := range tables {
		d := src[44+20*i:]
		t := woffTable{
			tag:          string(d[0:4]),
			offset:       be.Uint32(d[4:]),
			length:       be.Uint32(d[8:]),
			origLength:   be.Uint32(d[12:]),
			origChecksum: be.Uint32(d[16:]),
		}
		if length < t.length || length-t.length < t.offset {
			return nil, fmt.Errorf("table %q extends beyond file size", t.tag)
		} else if t.origLength < t.length {
			return nil, fmt.Errorf("compressed table %q is larger than decompressed table", t.tag)
		}
		tables[i] = t
	}

	// the sfnt table directory must be sorted by tag
	slices.SortFunc(tables, func(a, b woffTable) int {
		return cmp.Compare(a.tag, b.tag)
	})
	for i := 1; i < len(tables); i++ {
		if tables[i].tag == tables[i-1].tag {
			return nil, fmt.Errorf("duplicate table %q", tables[i].tag)
		}
	}

	// table data is written in the order it is stored in the WOFF so things
	// like DSIG stay valid
	order := make([]int, len(tables))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(i, j int) int {
		a, b := tables[i], tables[j]
		return cmp.Or(cmp.Compare(a.offset, b.offset), cmp.Compare(a.length, b.length))
	})
	end := frontSize
	size := uint64(12 + 16*uint32(numTables))
	for _, i := range order {
		t := &tables[i]
		if t.length != 0 {
			if t.offset < end {
				return nil, fmt.Errorf("table %q overlaps", t.tag)
			}
			end = t.offset + t.length
		}
		end = t.offset + t.length
		t.sfntOffset = uint32(size)
		size += pad4(uint64(t.origLength))
		if size > math.MaxUint32 {
			return nil, fmt.Errorf("invalid font data")
		}
	}
	if size > math.MaxInt {
		return nil, fmt.Errorf("font is too large")
	}

	h, ok := slices.BinarySearchFunc(tables, "head", func(t woffTable, tag string) int {
		return cmp.Compare(t.tag, tag)
	})
	if !ok || tables[h].origLength < 12 {
		return nil, fmt.Errorf("missing or short head table")
	}

	return &font{
		flavor: flavor,
		tables: tables,
		order:  order,
		head:   tables[h].sfntOffset,
		size:   int(size),
	}, nil
}

// write builds the sfnt font in w, which must have length f.size.
func (f *font) write(w, src []byte) error {
	var bad bool
	for _, i := range f.order {
		if ck, err := writeTable(w, src, &f.tables[i]); err != nil {
			return err
		} else if ck != f.tables[i].origChecksum {
			bad = true
		}
	}
	f.writeDirectory(w)
	if bad {
		return ErrBadChecksum
	}
	return nil
}

func writeTable(w, src []byte, t *woffTable) (uint32, error) {
	data := src[t.offset : t.offset+t.length]
	end := t.sfntOffset + t.origLength
	dst := w[t.sfntOffset:end]
	if t.length == t.origLength {
		copy(dst, data)
	} else {
		r, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return 0, fmt.Errorf("table %q decompression failed: %w", t.tag, err)
		}
		n, err := readFull(r, dst)
		if err == nil {
			var extra [1]byte
			var m int
			m, err = readFull(r, extra[:])
			if m != 0 {
				return 0, fmt.Errorf("table %q decompressed data is longer than origLength", t.tag)
			}
		}
		switch {
		case err == io.EOF && n == len(dst):
		case err == io.EOF:
			return 0, fmt.Errorf("table %q decompressed data is shorter than origLength (%d < %d)", t.tag, n, t.origLength)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return 0, fmt.Errorf("table %q compressed stream is truncated", t.tag)
		default:
			return 0, fmt.Errorf("table %q decompression failed: %w", t.tag, err)
		}
	}
	padded := t.sfntOffset + uint32(pad4(uint64(t.origLength)))
	clear(w[end:padded])

	if t.tag == "head" {
		// the head checksum is calculated with checksumAdjustment zeroed
		clear(dst[8:12])
	}
	return checksum(w[t.sfntOffset:padded]), nil
}

func (f *font) writeDirectory(w []byte) {
	be := binary.BigEndian
	numTables := uint16(len(f.tables))
	var searchRange uint16 = 1
	var entrySelector uint16
	for searchRange*2 <= numTables {
		searchRange *= 2
		entrySelector++
	}
	searchRange *= 16
	rangeShift := numTables*16 - searchRange

	h := w[:0]
	h = append(h, f.flavor...)
	h = be.AppendUint16(h, numTables)
	h = be.AppendUint16(h, searchRange)
	h = be.AppendUint16(h, entrySelector)
	h = be.AppendUint16(h, rangeShift)
	for _, t := range f.tables {
		h = append(h, t.tag...)
		h = be.AppendUint32(h, t.origChecksum)
		h = be.AppendUint32(h, t.sfntOffset)
		h = be.AppendUint32(h, t.origLength)
	}

	// the original checksumAdjustment is not validated since it is often wrong
	be.PutUint32(w[f.head+8:], 0xB1B0AFBA-checksum(w))
}

func pad4(n uint64) uint64 {
	return (n + 3) &^ 3
}

func checksum(b []byte) uint32 {
	if len(b)%4 != 0 {
		panic("data not multiple of four bytes")
	}
	var sum uint32
	for i := 0; i < len(b); i += 4 {
		sum += binary.BigEndian.Uint32(b[i:])
	}
	return sum
}

func readFull(r io.Reader, b []byte) (int, error) {
	var n int
	for n < len(b) {
		m, err := r.Read(b[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
