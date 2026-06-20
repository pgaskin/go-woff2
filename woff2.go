package woff2

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"

	"github.com/pgaskin/go-woff2/internal"
)

//go:generate docker build --platform amd64 --progress plain --output . src

type Params struct {
	ExtendedMetadata string
	BrotliQuality    int
	AllowTransforms  bool
}

func EncodeMaxLength(ttf []byte, params *Params) (int, error) {
	x := newInstance(nil)
	p, err := x.copy(ttf)
	if err != nil {
		return 0, err
	}
	var m, mn uint32
	if params != nil && params.ExtendedMetadata != "" {
		mp, err := x.copy([]byte(params.ExtendedMetadata))
		if err != nil {
			return 0, err
		}
		m, mn = mp, uint32(len(params.ExtendedMetadata))
	}
	return int(x.m.Xwoff2_encode_size_max(int32(p), int32(uint32(len(ttf))), int32(m), int32(mn))), nil
}

func Encode(ttf []byte, params *Params) ([]byte, error) {
	x := newInstance(nil)
	p, err := x.copy(ttf)
	if err != nil {
		return nil, err
	}
	var m, mn uint32
	if params != nil && params.ExtendedMetadata != "" {
		mp, err := x.copy([]byte(params.ExtendedMetadata))
		if err != nil {
			return nil, err
		}
		m, mn = mp, uint32(len(params.ExtendedMetadata))
	}
	on := int(x.m.Xwoff2_encode_size_max(int32(p), int32(uint32(len(ttf))), int32(m), int32(mn)))
	op, err := x.alloc(on)
	if err != nil {
		return nil, err
	}
	var bq int32
	if params != nil {
		bq = int32(params.BrotliQuality)
	}
	var tr int32
	if params != nil && params.AllowTransforms {
		tr = 1
	}
	n, ok := x.m.Xwoff2_encode(int32(p), int32(uint32(len(ttf))), int32(op), int32(on), int32(m), int32(mn), bq, tr)
	if ok == 0 {
		return nil, x.fail("woff2 encode failed")
	}
	ob, err := x.mem(op, uint32(n))
	if err != nil {
		return nil, err
	}
	return slices.Clone(ob), nil
}

func DecodeLength(woff2 []byte) (int, error) {
	x := newInstance(nil)
	p, err := x.copy(woff2)
	if err != nil {
		return 0, err
	}
	n := x.m.Xwoff2_decode_size(int32(p), int32(uint32(len(woff2))))
	if n == 0 {
		return 0, x.fail("woff2 decode failed")
	}
	return int(n), nil
}

func Decode(w interface {
	io.Writer
	io.WriterAt
}, woff2 []byte) error {
	x := newInstance(w)
	p, err := x.copy(woff2)
	if err != nil {
		return err
	}
	r := x.m.Xwoff2_decode(int32(p), int32(uint32(len(woff2))))
	if r == 0 {
		return x.fail("woff2 decode failed")
	}
	return nil
}

func DecodeBytes(woff2 []byte) ([]byte, error) {
	var b buffer
	if err := Decode(&b, woff2); err != nil {
		return nil, err
	}
	return b.b.Bytes(), nil
}

type buffer struct {
	b bytes.Buffer
}

func (b *buffer) Write(p []byte) (int, error) {
	return b.b.Write(p)
}

func (b *buffer) WriteAt(p []byte, off int64) (int, error) {
	if off+int64(len(p)) > int64(b.b.Len()) {
		return 0, errors.New("write past end of file")
	}
	copy(b.b.Bytes()[off:], p)
	return int(len(p)), nil
}

type writer interface {
	io.Writer
	io.WriterAt
}

type instance struct {
	w writer
	m *internal.Module
	e error
	r strings.Builder // captured stderr
}

func (x *instance) fail(msg string) error {
	if x.e != nil {
		return x.e
	}
	if out := strings.TrimSpace(x.r.String()); out != "" {
		return fmt.Errorf("%s (%q)", msg, out)
	}
	return errors.New(msg)
}

func newInstance(w writer) *instance {
	x := new(instance)
	x.w = w
	x.m = internal.New(x)
	return x
}

func (x *instance) copy(b []byte) (uint32, error) {
	p, err := x.alloc(len(b))
	if err != nil {
		return 0, err
	}
	a, err := x.mem(p, uint32(len(b)))
	if err != nil {
		return 0, err
	}
	if len(a) != len(b) {
		panic("wtf")
	}
	copy(a, b)
	return p, nil
}

func (x *instance) alloc(n int) (uint32, error) {
	if n >= math.MaxUint32 {
		return 0, errors.New("input too large")
	}
	p := x.m.Xmalloc(int32(n))
	if p == 0 {
		return 0, errors.New("wasm out of memory")
	}
	return uint32(p), nil
}

func (x *instance) mem(ptr, n uint32) ([]byte, error) {
	if n == 0 {
		return nil, nil
	}
	if ptr == 0 {
		return nil, errors.New("null wasm pointer")
	}
	b := *x.m.Xmemory().Slice()
	if ptr > uint32(len(b)) || n > uint32(len(b))-ptr {
		return nil, errors.New("invalid wasm pointer")
	}
	return b[ptr : ptr+n], nil
}

func (x *instance) Xwrite(ptr, n int32) int32 {
	if x.e != nil {
		return 0
	}
	if x.w == nil {
		x.e = errors.New("no writer")
		return 0
	}
	b, err := x.mem(uint32(ptr), uint32(n))
	if err != nil {
		x.e = err
		return 0
	}
	w, err := x.w.Write(b)
	if err != nil {
		x.e = err
		return 0
	}
	if int32(w) != n {
		x.e = io.ErrShortWrite
		return 0
	}
	return 1
}

func (x *instance) Xwrite_err(ptr, n int32) {
	b, err := x.mem(uint32(ptr), uint32(n))
	if err != nil {
		return
	}
	x.r.Write(b)
}

func (x *instance) Xwrite_at(ptr, off, n int32) int32 {
	if x.e != nil {
		return 0
	}
	if x.w == nil {
		x.e = errors.New("no writer")
		return 0
	}
	b, err := x.mem(uint32(ptr), uint32(n))
	if err != nil {
		x.e = err
		return 0
	}
	w, err := x.w.WriteAt(b, int64(off))
	if err != nil {
		x.e = err
		return 0
	}
	if int32(w) != n {
		x.e = io.ErrShortWrite
		return 0
	}
	return 1
}
