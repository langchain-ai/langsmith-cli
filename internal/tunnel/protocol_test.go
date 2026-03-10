package tunnel

import (
	"bytes"
	"testing"
)

func TestConnectHeaderRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		port uint16
	}{
		{"low port", 80},
		{"standard port", 5432},
		{"high port", 65535},
		{"zero port", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteConnectHeader(&buf, tt.port); err != nil {
				t.Fatalf("WriteConnectHeader: %v", err)
			}
			if buf.Len() != connectHeaderSize {
				t.Fatalf("expected %d bytes, got %d", connectHeaderSize, buf.Len())
			}

			hdr, err := ReadConnectHeader(&buf)
			if err != nil {
				t.Fatalf("ReadConnectHeader: %v", err)
			}
			if hdr.Version != ProtocolVersion {
				t.Errorf("version: expected %d, got %d", ProtocolVersion, hdr.Version)
			}
			if hdr.Port != tt.port {
				t.Errorf("port: expected %d, got %d", tt.port, hdr.Port)
			}
		})
	}
}

func TestConnectHeaderBinaryFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteConnectHeader(&buf, 8080); err != nil { // 0x1F90
		t.Fatalf("WriteConnectHeader: %v", err)
	}

	b := buf.Bytes()
	if b[0] != 0x01 {
		t.Errorf("version byte: expected 0x01, got 0x%02x", b[0])
	}
	if b[1] != 0x1F {
		t.Errorf("port high byte: expected 0x1F, got 0x%02x", b[1])
	}
	if b[2] != 0x90 {
		t.Errorf("port low byte: expected 0x90, got 0x%02x", b[2])
	}
}

func TestReadConnectHeaderTruncated(t *testing.T) {
	buf := bytes.NewReader([]byte{0x01, 0x00}) // only 2 bytes, need 3
	if _, err := ReadConnectHeader(buf); err == nil {
		t.Fatal("expected error on truncated input")
	}
}

func TestStatusRoundTrip(t *testing.T) {
	statuses := []byte{StatusOK, StatusPortNotAllowed, StatusDialFailed, StatusUnsupportedVersion}
	for _, s := range statuses {
		var buf bytes.Buffer
		if err := WriteStatus(&buf, s); err != nil {
			t.Fatalf("WriteStatus(0x%02x): %v", s, err)
		}

		got, err := ReadStatus(&buf)
		if err != nil {
			t.Fatalf("ReadStatus: %v", err)
		}
		if got != s {
			t.Errorf("expected 0x%02x, got 0x%02x", s, got)
		}
	}
}

func TestReadStatusEmpty(t *testing.T) {
	buf := bytes.NewReader([]byte{})
	if _, err := ReadStatus(buf); err == nil {
		t.Fatal("expected error on empty input")
	}
}
