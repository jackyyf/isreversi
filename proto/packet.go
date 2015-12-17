package proto

import (
	"github.com/agl/ed25519"
	"encoding/binary"
	"io"
	"errors"
)

var ErrNil = errors.New("Can't parse packet into nil")
var ErrType = errors.New("Invalid frame type.")
var ErrPayload = errors.New("Invalid format of payload.")

type RawPacket struct {
	frametype uint16
	payload []byte
	signature *[ed25519.SignatureSize]byte
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
	frametype := binary.BigEndian.Uint16(buf[:])
	payload := make([]byte, length)
	if _, err = io.ReadFull(reader, payload); err != nil {
		return
	}
	signature := new([ed25519.SignatureSize]byte)
	if _, err = io.ReadFull(reader, (*signature)[:]); err != nil {
		return
	}
	return &RawPacket{
		frametype: frametype,
		payload: payload,
		signature: signature,
	}, nil
}

func (p *RawPacket) Sign(privk *[ed25519.PrivateKeySize]byte) {
	length := uint16(len(p.payload))
	buf := make([]byte, length + 4)
	binary.BigEndian.PutUint16(buf, length)
	binary.BigEndian.PutUint16(buf[2:], p.frametype)
	copy(buf[4:], p.payload)
	p.signature = ed25519.Sign(privk, buf)
}

func (p *RawPacket) Validate(pubk *[ed25519.PublicKeySize]byte) bool {
	if p.signature == nil {
		return false
	}
	length := uint16(len(p.payload))
	buf := make([]byte, length + 4)
	binary.BigEndian.PutUint16(buf, length)
	binary.BigEndian.PutUint16(buf[2:], p.frametype)
	copy(buf[4:], p.payload)
	return ed25519.Verify(pubk, buf, p.signature)
}

func (p *RawPacket) Bytes() []byte {
	if p.signature == nil {
		return nil
	}
	length := uint16(len(p.payload))
	buf := make([]byte, length + 4 + ed25519.SignatureSize)
	binary.BigEndian.PutUint16(buf, length)
	binary.BigEndian.PutUint16(buf[2:], p.frametype)
	copy(buf[4:], p.payload)
	copy(buf[4+length:], (*p.signature)[:])
	return buf
}

type Packet interface {
	Packet() *RawPacket
	Payload() []byte
	Parse(*RawPacket) error
}

type Player struct {
	name string
	win uint32
	lose uint32
}

type Room struct {
	id uint16
	title string
	players [2]string
}

type ClientHello struct {
	pubkey *[ed25519.PublicKeySize]byte
	challenge *[32]byte
}

func (p *ClientHello) Payload() []byte {
	if p.pubkey == nil || p.challenge == nil {
		return nil
	}
	ret := make([]byte, ed25519.PublicKeySize + 32)
	copy(ret, *p.pubkey)
	copy(ret[ed25519.PublicKeySize:], *p.challenge)
}

func (p *ClientHello) Packet() *RawPacket {
	payload := p.Payload()
	if payload == nil {
		return nil
	}
	return &RawPacket {
		frametype: 0,
		payload: payload,
	}
}

func (p *ClientHello) Parse(pkt *RawPacket) error {
	if p == nil {
		return ErrNil
	}
	if pkt.frametype != 0 {
		return ErrType
	}
	if len(pkt.payload) != 32 + ed25519.PublicKeySize {
		return ErrPayload
	}
	p.pubkey = new([ed25519.PublicKeySize]byte)
	p.challenge = new([32]byte)
	copy((*p.pubkey)[:], pkt.payload)
	copy((*p.challenge)[:], pkt.payload[ed25519.PublicKeySize:])
	return nil
}

type ServerHello struct {
	pubkey *[ed25519.PublicKeySize]byte
	clientch *[32]byte
	serverch *[32]byte
}

type ClientConfirm struct {
	challenge *[32]byte
}

type ServerConfirm struct {
	fresh uint8
}

type ClientRegister struct {
	username string
}

type ServerRegister struct {
	result uint8
}

type Quit struct {
	reason string
}

type ClientRoomList struct {
}

type ServerRoomList struct {
	data []byte
}

type ClientPlayerList struct {
}

type ServerPlayerList struct {
	data []byte
}

type ClientJoinGame struct {
	id uint16
}

type ServerJoinGame struct {
	result uint8
}

type ClientLeaveGame struct {
}

type ServerLeaveGame struct {
}

type ClientGameRestart struct {
}

type ClientPlace struct {
	xy uint8
}

type ServerBoardUpdate struct {
	placed uint64
	color uint64
}

type ChatMessage struct {
	user string
	message string
}
