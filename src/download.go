//go:build ignore

package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	mustFetch("google/woff2", "1c69169e9e1811dccd6c54c532fedda300233968", "woff2")
	mustFetch("google/brotli", "533843e3546cd24c8344eaa899c6b0b681c8d222", "brotli")
}

func mustFetch(repo, commit, dest string) {
	if err := fetch(repo, commit, dest); err != nil {
		fmt.Fprintf(os.Stderr, "error: download %s: %v\n", repo, err)
		os.Exit(1)
	}
}

func fetch(repo, commit, dest string) error {
	resp, err := http.Get("https://github.com/" + repo + "/archive/" + commit + ".tar.gz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("response status %d", resp.StatusCode)
	}

	if err := os.RemoveAll(dest); err != nil {
		return err
	}

	if err := os.Mkdir(dest, 0755); err != nil {
		return err
	}

	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}

	if err := root.WriteFile("COMMIT", []byte(commit), 0644); err != nil {
		return err
	}

	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}

	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		rel := h.Name
		if _, rest, ok := strings.Cut(rel, "/"); ok {
			rel = rest // remove top-level dir
		}
		if rel == "" {
			continue
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(rel, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := root.MkdirAll(filepath.Dir(rel), 0755); err != nil {
				return err
			}
			f, err := root.OpenFile(rel, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}
