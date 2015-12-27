package common

import (
	"github.com/jackyyf/isreversi/proto"
	"github.com/agl/ed25519"
	"crypto/rand"
	"net"
	"fmt"
	"errors"
	"encoding/binary"
)

var signtestmsg = []byte("jackyyf.isreversi")
var ErrInvalidKey = errors.New("Invalid key: pubkey and privkey does not match.")

type User struct {
	Pubkey	*[ed25519.PublicKeySize]byte
	Privkey	*[ed25519.PrivateKeySize]byte
	Name	string
	Game	*Game
	Conn	net.Conn
	Kicked	bool
	RPubKey *[ed25519.PublicKeySize]byte
	Ready	bool
}

func NewUser() *User {
	ret := new(User)
	return ret
}

func (u *User) GenerateKeys() (err error) {
	u.Pubkey, u.Privkey, err = ed25519.GenerateKey(rand.Reader)
	return
}

func (u *User) SetKeys(pubkey *[ed25519.PublicKeySize]byte, privkey *[ed25519.PrivateKeySize]byte) (err error) {
	sign := ed25519.Sign(privkey, signtestmsg)
	if ed25519.Verify(pubkey, signtestmsg, sign) {
		return nil
	}
	return ErrInvalidKey
}

func (u *User) Join(g *Game) uint8 { // Result code
	if g == nil {
		return 1
	}
	if u.Game != nil {
		return 3
	}
	g.Mutex.Lock()
	defer g.Mutex.Unlock()
	if g.Black != nil && g.White != nil {
		return 2
	}
	u.Game = g
	defer u.refreshBoard()
	if g.Black == nil {
		g.Black = u
		if g.White != nil {
			g.White.SendMessage("", fmt.Sprintf("User %s joined the game!", u.Name))
		}
		return 0
	}
	if g.White == nil {
		g.White = u
		if g.Black != nil {
			g.Black.SendMessage("", fmt.Sprintf("User %s joined the game!", u.Name))
		}
		return 0
	}
	panic("Should never reach here!!")
}

func (u *User) Leave() {
	if u.Game == nil {
		return
	}
	g := u.Game
	u.Game = nil
	g.Mutex.Lock()
	defer g.Mutex.Unlock()
	g.Turn = 0
	if g.Black == u {
		g.Black = nil
		if g.White != nil {
			g.White.SendMessage("", fmt.Sprintf("User %s left the game!", u.Name))
		}
		return
	}
	if g.White == u {
		g.White = nil
		if g.Black != nil {
			g.Black.SendMessage("", fmt.Sprintf("User %s left the game!", u.Name))
		}
		return
	}
	panic("Should not reach here!")
}

func (u *User) Kick(reason string) {
	u.Leave()
	u.Kicked = true
	buf := make([]byte, len(reason) + 2)
	b := buf
	buf, _ = proto.WriteString(buf, reason)
	pkt := proto.NewPacket(5, b)
	pkt.Sign(u.Privkey)
	u.Conn.Write(pkt.Bytes())
	u.Conn.Close()

}

func (u *User) Restart() bool {
	if u.Game == nil {
		return false
	}
	u.Ready = true
	if u.Game.TryStart() {
		u.Game.Restart()
		go u.Game.Black.refreshBoard()
		go u.Game.White.refreshBoard()
	}
	return true
}

func (u *User) SendMessage(from string, msg string) {
	buf := make([]byte, len(from) + len(msg) + 4)
	b := buf
	buf, err := proto.WriteString(buf, from)
	buf, err = proto.WriteString(buf, msg)
	pkt := proto.NewPacket(13, b)
	pkt.Sign(u.Privkey)
	_, err = u.Conn.Write(pkt.Bytes())
	if err != nil {
		u.Kicked = true
		u.Conn.Close()
		return
	}
}

func (u *User) refreshBoard() {
	if u.Game == nil {
		return
	}
	var placed, color uint64
	for i := 0; i < 8; i ++ {
		for j := 0; j < 8; j ++ {
			this := uint((i << 3) | j)
			if u.Game.Board[i][j] != 0 {
				placed |= uint64(1) << this
				if u.Game.Board[i][j] == 2 {
					color |= uint64(1) << this
				}
			}
		}
	}
	black := "[Empty]"
	white := "[Empty]"
	if u.Game.Black != nil {
		black = u.Game.Black.Name
	}
	if u.Game.White != nil {
		white = u.Game.White.Name
	}
	buf := make([]byte, 16384)
	binary.BigEndian.PutUint64(buf, placed)
	binary.BigEndian.PutUint64(buf[8:], color)
	buf[16] = byte(u.Game.Turn)
	b := buf[17:]
	b, _ = proto.WriteString(b, white)
	b, _ = proto.WriteString(b, black)
	length := 16384 - len(b)
	pkt := proto.NewPacket(12, buf[:length])
	pkt.Sign(u.Privkey)
	_, err := u.Conn.Write(pkt.Bytes())
	if err != nil {
		u.Kicked = true
		u.Conn.Close()
		return
	}
}
