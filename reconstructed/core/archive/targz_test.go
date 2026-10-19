package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func makeTarGz(t *testing.T, entries map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "a.tar.gz")
	if err := os.WriteFile(src, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return src
}

func TestExtractTarGz(t *testing.T) {
	src := makeTarGz(t, map[string]string{
		"ppanel-server":   "#!/bin/sh\n",
		"etc/config.yaml": "k: v\n",
	})
	dest := t.TempDir()
	if err := ExtractTarGz(src, dest); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"ppanel-server":   "#!/bin/sh\n",
		"etc/config.yaml": "k: v\n",
	} {
		got, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(got) != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestExtractTarGzRejectsTraversal(t *testing.T) {
	src := makeTarGz(t, map[string]string{"../escape": "x"})
	if err := ExtractTarGz(src, t.TempDir()); err == nil {
		t.Fatal("expected path-traversal rejection")
	}
}
