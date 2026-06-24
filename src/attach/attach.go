// Package attach loads and validates local files referenced via @path syntax,
// encoding them into backend attachments before any request is sent.
package attach

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/spf13/viper"

	"metaphox/underdash/backend"
)

// defaultMaxBytes is the per-file size cap, kept well under the provider's
// per-request limit. Override via the attach.max_bytes config key.
const defaultMaxBytes = 5 << 20 // 5 MiB

// maxBytes returns the per-file size cap.
func maxBytes() int64 {
	if v := viper.GetInt64("attach.max_bytes"); v > 0 {
		return v
	}
	return defaultMaxBytes
}

// Load reads, validates, and encodes each path into a backend.Attachment. It
// fails fast on the first unreadable, oversized, or unsupported file so the
// caller can abort before spending a backend request.
func Load(paths []string) ([]backend.Attachment, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	atts := make([]backend.Attachment, 0, len(paths))
	for _, p := range paths {
		a, err := loadOne(p)
		if err != nil {
			return nil, err
		}
		atts = append(atts, a)
	}
	return atts, nil
}

func loadOne(path string) (backend.Attachment, error) {
	info, err := os.Stat(path)
	if err != nil {
		return backend.Attachment{}, fmt.Errorf("attachment %s: %w", path, err)
	}
	if info.IsDir() {
		return backend.Attachment{}, fmt.Errorf("attachment %s: is a directory", path)
	}
	if max := maxBytes(); info.Size() > max {
		return backend.Attachment{}, fmt.Errorf("attachment %s: %d bytes exceeds limit of %d bytes (raise with attach.max_bytes)", path, info.Size(), max)
	}

	data, err := os.ReadFile(path) //nolint:gosec // path is a user-supplied attachment
	if err != nil {
		return backend.Attachment{}, fmt.Errorf("attachment %s: %w", path, err)
	}

	kind, mediaType, err := classify(path, data)
	if err != nil {
		return backend.Attachment{}, err
	}

	att := backend.Attachment{
		Filename:  filepath.Base(path),
		Kind:      kind,
		MediaType: mediaType,
	}
	if kind == "text" {
		// Text files travel as-is; no base64 needed.
		att.Data = string(data)
	} else {
		att.Data = base64.StdEncoding.EncodeToString(data)
	}
	return att, nil
}

// classify determines the attachment kind and media type from the file
// extension first, falling back to content sniffing. It returns an error for
// types no backend can accept.
func classify(path string, data []byte) (kind, mediaType string, err error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image", "image/png", nil
	case ".jpg", ".jpeg":
		return "image", "image/jpeg", nil
	case ".gif":
		return "image", "image/gif", nil
	case ".webp":
		return "image", "image/webp", nil
	case ".pdf":
		return "document", "application/pdf", nil
	}

	// No decisive extension — sniff the leading bytes.
	sniff := data
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	detected := http.DetectContentType(sniff)
	base := strings.SplitN(detected, ";", 2)[0]

	switch {
	case strings.HasPrefix(base, "image/"):
		switch base {
		case "image/png", "image/jpeg", "image/gif", "image/webp":
			return "image", base, nil
		default:
			return "", "", fmt.Errorf("attachment %s: unsupported image type %q", filepath.Base(path), base)
		}
	case base == "application/pdf":
		return "document", "application/pdf", nil
	case strings.HasPrefix(base, "text/"), utf8.Valid(data):
		return "text", "text/plain", nil
	default:
		return "", "", fmt.Errorf("attachment %s: unsupported file type (%s)", filepath.Base(path), base)
	}
}
