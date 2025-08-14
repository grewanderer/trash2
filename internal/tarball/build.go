package tarball

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"

	"wisp/internal/render/uci"
)

// Build собирает tar.gz из рендеренных UCI-файлов и extra-артефактов.
// Возвращает архив и sha256 в hex.
func Build(files []uci.File, extra map[string][]byte) ([]byte, string, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	add := func(name string, data []byte, mode int64) error {
		// sanitize path: no leading slash, clean, no ..
		name = strings.TrimLeft(name, "/")
		name = filepath.ToSlash(filepath.Clean(name))
		hdr := &tar.Header{Name: name, Mode: mode, Size: int64(len(data))}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}

	for _, f := range files {
		if err := add(f.Path, f.Data, f.Mode); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return nil, "", err
		}
	}
	for p, b := range extra {
		if err := add(p, b, 0644); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return nil, "", err
		}
	}
	_ = tw.Close()
	_ = gz.Close()

	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:]), nil
}
