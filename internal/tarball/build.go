package tarball

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"wisp/internal/render/uci"
)

// Build собирает tar.gz из рендеренных UCI-файлов и extra-артефактов.
// Возвращает архив и sha256 в hex.
func Build(files []uci.File, extra map[string][]byte) ([]byte, string, error) {
	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)
	// детерминируем gzip-заголовок
	gz.Name = ""
	gz.Comment = ""
	gz.ModTime = time.Unix(0, 0)

	tw := tar.NewWriter(gz)

	add := func(name string, data []byte, mode int64) error {
		// sanitize path: no leading slash, clean, unix slashes
		name = strings.TrimLeft(name, "/")
		name = filepath.ToSlash(filepath.Clean(name))
		if name == "" || name == "." {
			return nil
		}
		hdr := &tar.Header{
			Name:    name,
			Mode:    mode,
			Size:    int64(len(data)),
			ModTime: time.Unix(0, 0), // фиксируем время в tar-заголовке
			// по желанию можно зафиксировать и владельцев:
			Uid: 0, Gid: 0, Uname: "", Gname: "",
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}

	// 1) канонический порядок uci-файлов
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	for _, f := range files {
		mode := int64(f.Mode)
		if mode == 0 {
			mode = 0644
		}
		if err := add(f.Name, f.Data, mode); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return nil, "", err
		}
	}

	// 2) канонический порядок extra (map → сортированные ключи)
	if len(extra) > 0 {
		keys := make([]string, 0, len(extra))
		for k := range extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if err := add(k, extra[k], 0644); err != nil {
				_ = tw.Close()
				_ = gz.Close()
				return nil, "", err
			}
		}
	}

	_ = tw.Close()
	_ = gz.Close()

	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:]), nil
}
