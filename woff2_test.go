package woff2

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	fonts := []string{
		"SourceSans3VF-Upright.ttf",
		"SourceSans3VF-Upright.otf",
	}
	for _, font := range fonts {
		t.Run(font, func(t *testing.T) {
			ttf0, err := os.ReadFile(filepath.Join("testdata", font))
			if err != nil {
				t.Fatalf("read input font: %v", err)
			}

			// ttf0 -> woff1 (note: woff2 may normalize the tables)
			woff1, err := Encode(ttf0, nil)
			if err != nil {
				t.Fatalf("encode ttf0: %v", err)
			}
			if len(woff1) == 0 {
				t.Fatal("encode ttf0: empty output")
			}
			if !bytes.HasPrefix(woff1, []byte("wOF2")) {
				t.Fatalf("encode ttf0: missing wOF2 signature, got %x", woff1[:min(4, len(woff1))])
			}

			// woff1 -> ttf1
			ttf1, err := DecodeBytes(woff1)
			if err != nil {
				t.Fatalf("decode woff1: %v", err)
			}
			if len(ttf1) == 0 {
				t.Fatal("decode woff1: empty output")
			}

			// DecodeLength should match the actual decoded size
			if n, err := DecodeLength(woff1); err != nil {
				t.Fatalf("decode length woff1: %v", err)
			} else if n != len(ttf1) {
				t.Errorf("DecodeLength: got %d, but decoded %d bytes", n, len(ttf1))
			}

			// ttf1 -> woff2
			woff2, err := Encode(ttf1, nil)
			if err != nil {
				t.Fatalf("encode ttf1: %v", err)
			}

			// woff2 -> ttf2
			ttf2, err := DecodeBytes(woff2)
			if err != nil {
				t.Fatalf("decode woff2: %v", err)
			}

			// re-encoding the normalized font must be stable
			if !bytes.Equal(woff1, woff2) {
				t.Errorf("woff1 != woff2 (re-encode not stable): %d vs %d bytes", len(woff1), len(woff2))
			}

			// re-decoding must be stable
			if !bytes.Equal(ttf1, ttf2) {
				t.Errorf("ttf1 != ttf2 (re-decode not stable): %d vs %d bytes", len(ttf1), len(ttf2))
			}
		})
	}
}

// TestInvalid checks that malformed input is rejected with an error rather than
// silently producing garbage.
func TestInvalid(t *testing.T) {
	inputs := map[string][]byte{
		"nil":   nil,
		"empty": {},
		"junk":  []byte("not a font"),
		"zeros": make([]byte, 256),
	}
	t.Run("Encode", func(t *testing.T) {
		for name, in := range inputs {
			t.Run(name, func(t *testing.T) {
				if out, err := Encode(in, nil); err == nil {
					t.Fatalf("Encode: got %d bytes, wanted error", len(out))
				} else {
					t.Logf("%v", err)
				}
			})
		}
	})
	t.Run("Decode", func(t *testing.T) {
		for name, in := range inputs {
			t.Run(name, func(t *testing.T) {
				if n, err := DecodeLength(in); err == nil {
					// note: this doesn't validate the input, so there may be
					// some more complex cases where this returns nil on an
					// invalid font, but DecodeBytes returns an error
					t.Fatalf("DecodeLength: got %d bytes, wanted error", n)
				}
				if out, err := DecodeBytes(in); err == nil {
					t.Fatalf("DecodeBytes: got %d bytes, wanted error", len(out))
				} else {
					t.Logf("%v", err)
				}
			})
		}
	})
}

func TestWriteError(t *testing.T) {
	ttf, err := os.ReadFile(filepath.Join("testdata", "SourceSans3VF-Upright.ttf"))
	if err != nil {
		t.Fatalf("read input font: %v", err)
	}
	woff, err := Encode(ttf, nil)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := errors.New("test")
	err = Decode(&errWriter{err: want}, woff)
	if err == nil {
		t.Fatal("decode: expected error, got none")
	}
	if !errors.Is(err, want) {
		t.Fatalf("decode: got incorrect error: %v", err)
	}
}

// errWriter fails on the second Write/WriteAt call with a sentinel error, and
// behaves like a normal in-memory writer for all other calls.
type errWriter struct {
	err   error
	calls int
	buf   []byte
}

func (w *errWriter) fail() bool {
	w.calls++
	return w.calls == 2
}

func (w *errWriter) Write(p []byte) (int, error) {
	if w.fail() {
		return 0, w.err
	}
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *errWriter) WriteAt(p []byte, off int64) (int, error) {
	if w.fail() {
		return 0, w.err
	}
	if off+int64(len(p)) > int64(len(w.buf)) {
		return 0, errors.New("write past end of file")
	}
	copy(w.buf[off:], p)
	return len(p), nil
}
