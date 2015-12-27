package proto

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/agl/ed25519"
)

var ErrNil = errors.New("Can't parse packet into nil")
var ErrType = errors.New("Invalid frame type.")
var ErrPayload = errors.New("Invalid format of Payload.")
var ErrShortBuf = errors.New("Buffer too short.")

type RawPacket struct {
	Frametype uint16
	Payload   []byte
	Signature *[ed25519.SignatureSize]byte
}

func ReadPacket(reader io.Reader) (pkt *RawPacket, err error) {
	var buf [2]byte
	if _, err = io.ReadFull(reader, buf[:]); err != nil {
		return
	}
	length := binary.BigEndian.Uint16(buf[:])
	if _, err = io.ReadFull(reader, buf[:]); err != nil {
		return
	}
	Frametype := binary.BigEndian.Uint16(buf[:])
	Payload := make([]byte, length)
	if _, err = io.ReadFull(reader, Payload); err != nil {
		return
	}
	Signature := new([ed25519.SignatureSize]byte)
	if _, err = io.ReadFull(reader, (*Signature)[:]); err != nil {
		return
	}
	return &RawPacket{
		Frametype: Frametype,
		Payload:   Payload,
		Signature: Signature,
	}, nil
}

func (p *RawPacket) Sign(privk *[ed25519.PrivateKeySize]byte) {
	length := uint16(len(p.Payload))
	buf := make([]byte, length+4)
	binary.BigEndian.PutUint16(buf, length)
	binary.BigEndian.PutUint16(buf[2:], p.Frametype)
	copy(buf[4:], p.Payload)
	p.Signature = ed25519.Sign(privk, buf)
}

func (p *RawPacket) Validate(pubk *[ed25519.PublicKeySize]byte) bool {
	if p.Signature == nil {
		return false
	}
	length := uint16(len(p.Payload))
	buf := make([]byte, length+4)
	binary.BigEndian.PutUint16(buf, length)
	binary.BigEndian.PutUint16(buf[2:], p.Frametype)
	copy(buf[4:], p.Payload)
	return ed25519.Verify(pubk, buf, p.Signature)
}

func NewPacket(frametype uint16, payload []byte) *RawPacket {
	return &RawPacket {
		Frametype: frametype,
		Payload: payload,
	}
}

func (p *RawPacket) Bytes() []byte {
	if p.Signature == nil {
		return nil
	}
	length := uint16(len(p.Payload))
	buf := make([]byte, length+4+ed25519.SignatureSize)
	binary.BigEndian.PutUint16(buf, length)
	binary.BigEndian.PutUint16(buf[2:], p.Frametype)
	copy(buf[4:], p.Payload)
	copy(buf[4+length:], (*p.Signature)[:])
	return buf
}

func ReadString(buf []byte) (s string, lft []byte, err error) {
	if len(buf) < 2 {
		return "", buf, ErrShortBuf
	}
	l := binary.BigEndian.Uint16(buf[:2])
	if uint16(len(buf) - 2) < l {
		return "", buf, ErrShortBuf
	}
	return string(buf[2:2+l]), buf[2+l:], nil
}

func ReadBytes(buf []byte) (b []byte, lft []byte, err error) {
	if len(buf) < 2 {
		return []byte{}, buf, ErrShortBuf
	}
	l := binary.BigEndian.Uint16(buf[:2])
	if uint16(len(buf) - 2) < l {
		return []byte{}, buf, ErrShortBuf
	}
	return buf[2:2+l], buf[2+l:], nil
}

func WriteString(buf []byte, s string) (lft []byte, err error) {
	if len(buf) < 2 + len(s) {
		return buf, ErrShortBuf
	}
	binary.BigEndian.PutUint16(buf, uint16(len(s)))
	copy(buf[2:], s[:])
	return buf[len(s) + 2:], nil
}

func WriteBytes(buf []byte, b []byte) (lft []byte, err error) {
	if len(buf) < 2 + len(b) {
		return buf, ErrShortBuf
	}
	binary.BigEndian.PutUint16(buf, uint16(len(b)))
	copy(buf[2:], b[:])
	return buf[len(b) + 2:], nil
}
