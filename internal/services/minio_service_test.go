package services

import (
	"testing"
)

func TestSniffMIME_JPEG(t *testing.T) {
	// JPEG magic bytes: FF D8 FF
	data := []byte{0xFF, 0xD8, 0xFF, 0x00, 0x00}
	mime := sniffMIME(data)
	if mime != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", mime)
	}
}

func TestSniffMIME_PNG(t *testing.T) {
	// PNG magic bytes: 89 50 4E 47 0D 0A 1A 0A
	data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mime := sniffMIME(data)
	if mime != "image/png" {
		t.Errorf("expected image/png, got %s", mime)
	}
}

func TestSniffMIME_WebP(t *testing.T) {
	// WebP magic bytes: RIFF????WEBP
	data := make([]byte, 12)
	data[0] = 'R'
	data[1] = 'I'
	data[2] = 'F'
	data[3] = 'F'
	// bytes 4-7 are file size (any value)
	data[8] = 'W'
	data[9] = 'E'
	data[10] = 'B'
	data[11] = 'P'
	mime := sniffMIME(data)
	if mime != "image/webp" {
		t.Errorf("expected image/webp, got %s", mime)
	}
}

func TestSniffMIME_Unknown(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02, 0x03}
	mime := sniffMIME(data)
	if mime != "" {
		t.Errorf("expected empty string for unknown format, got %s", mime)
	}
}

func TestSniffMIME_TooShort(t *testing.T) {
	data := []byte{0xFF, 0xD8, 0x00} // only 3 bytes, less than required 4
	mime := sniffMIME(data)
	if mime != "" {
		t.Errorf("expected empty string for too-short input, got %s", mime)
	}
}

func TestSniffMIME_Empty(t *testing.T) {
	mime := sniffMIME([]byte{})
	if mime != "" {
		t.Errorf("expected empty string for empty input, got %s", mime)
	}
}
