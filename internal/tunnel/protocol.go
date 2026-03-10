package tunnel

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	ProtocolVersion byte = 0x01

	StatusOK                 byte = 0x00
	StatusPortNotAllowed     byte = 0x01
	StatusDialFailed         byte = 0x02
	StatusUnsupportedVersion byte = 0x03

	connectHeaderSize = 3 // 1 byte version + 2 bytes port
)

// ConnectHeader is the first message sent on each yamux stream by the client.
type ConnectHeader struct {
	Version byte
	Port    uint16
}

// WriteConnectHeader writes a connect header to w.
func WriteConnectHeader(w io.Writer, port uint16) error {
	buf := [connectHeaderSize]byte{ProtocolVersion, 0, 0}
	binary.BigEndian.PutUint16(buf[1:], port)
	_, err := w.Write(buf[:])
	return err
}

// ReadConnectHeader reads a connect header from r.
func ReadConnectHeader(r io.Reader) (ConnectHeader, error) {
	var buf [connectHeaderSize]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return ConnectHeader{}, fmt.Errorf("read connect header: %w", err)
	}
	return ConnectHeader{
		Version: buf[0],
		Port:    binary.BigEndian.Uint16(buf[1:]),
	}, nil
}

// WriteStatus writes a single status byte to w.
func WriteStatus(w io.Writer, status byte) error {
	_, err := w.Write([]byte{status})
	return err
}

// ReadStatus reads a single status byte from r.
func ReadStatus(r io.Reader) (byte, error) {
	var buf [1]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, fmt.Errorf("read status: %w", err)
	}
	return buf[0], nil
}
