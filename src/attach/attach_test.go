package attach

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// minimal valid 1x1 PNG (signature + IHDR is enough for extension-based routing;
// we route PNG by extension, so any bytes suffice here).
var pngBytes = []byte("\x89PNG\r\n\x1a\nfake-png-body")

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_TextFileIsRawUTF8(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "notes.txt", []byte("hello world"))

	atts, err := Load([]string{p})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(atts) != 1 {
		t.Fatalf("got %d attachments, want 1", len(atts))
	}
	a := atts[0]
	if a.Kind != "text" || a.MediaType != "text/plain" {
		t.Errorf("kind/media = %q/%q, want text/text/plain", a.Kind, a.MediaType)
	}
	if a.Data != "hello world" {
		t.Errorf("Data = %q, want raw text", a.Data)
	}
	if a.Filename != "notes.txt" {
		t.Errorf("Filename = %q, want notes.txt", a.Filename)
	}
}

func TestLoad_ImageIsBase64WithoutNewlines(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "pic.png", pngBytes)

	atts, err := Load([]string{p})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	a := atts[0]
	if a.Kind != "image" || a.MediaType != "image/png" {
		t.Errorf("kind/media = %q/%q, want image/image/png", a.Kind, a.MediaType)
	}
	if a.Data == "" || strings.ContainsAny(a.Data, "\r\n") {
		t.Errorf("base64 data must be non-empty and newline-free, got %q", a.Data)
	}
}

func TestLoad_PDFByExtensionIsDocument(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "report.pdf", []byte("%PDF-1.4 ..."))

	atts, err := Load([]string{p})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if atts[0].Kind != "document" || atts[0].MediaType != "application/pdf" {
		t.Errorf("kind/media = %q/%q, want document/application/pdf", atts[0].Kind, atts[0].MediaType)
	}
}

func TestLoad_RejectsUnsupportedBinary(t *testing.T) {
	dir := t.TempDir()
	// A non-UTF-8 binary blob with no decisive extension → unsupported.
	p := writeFile(t, dir, "blob.bin", []byte{0x00, 0xff, 0xfe, 0x01, 0x02})

	_, err := Load([]string{p})
	if err == nil {
		t.Fatal("expected an error for unsupported binary, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error = %v, want it to mention 'unsupported'", err)
	}
}

func TestLoad_RejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "big.txt", []byte("0123456789"))

	viper.Set("attach.max_bytes", 4) // 4 bytes < 10
	t.Cleanup(func() { viper.Set("attach.max_bytes", 0) })

	_, err := Load([]string{p})
	if err == nil {
		t.Fatal("expected a size-limit error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Errorf("error = %v, want it to mention 'exceeds limit'", err)
	}
}

func TestLoad_MissingFileFailsFast(t *testing.T) {
	_, err := Load([]string{filepath.Join(t.TempDir(), "nope.txt")})
	if err == nil {
		t.Fatal("expected an error for a missing file, got nil")
	}
}

func TestLoad_EmptyReturnsNil(t *testing.T) {
	atts, err := Load(nil)
	if err != nil || atts != nil {
		t.Errorf("Load(nil) = %v, %v; want nil, nil", atts, err)
	}
}
