package socket

import (
	"crypto/rand"
	"io"
	"net"
	"sync"
	"time"

	"github.com/agl/ed25519"
)

const ChallengeSize = 16

type Packet struct {
	remoteaddr net.Addr
	pubkey     *[ed25519.PublicKeySize]byte
	id         uint16
	flags      uint8
	ack        uint16
	body       []byte
	signature  *[ed25519.SignatureSize]byte
}

type Conn struct {
	sndch      chan *Packet // Send channel
	rcvch      chan *Packet // Receive channel
	timer      *time.Timer
	mutex      sync.Mutex
	seq        uint16                       // My next seq
	ack        uint16                       // My last ack
	rcvwindow  [5]*Packet                   // Packet window for ack + 1 ~ ack + 5
	sndwindow  [5]*Packet                   // Packet window for seq - 5 ~ seq - 1
	pubkey     [ed25519.PublicKeySize]byte  // My pubkey
	privatekey [ed25519.PrivateKeySize]byte // My private key
	yourpubkey [ed25519.PublicKeySize]byte  // Your pubkey
	removeaddr net.Addr
}

// Only server side has half conn, client side can just block to wait on 4th ESTB packet.

// Note: when received CLHS, then the connection is considered open, even our ESTB lost, and client has retried with CLHS.
// Just resend ESTB and everything is fine :)
type HalfConn struct {
	challenge  [ChallengeSize]byte
	yourpubkey [ed25519.PublicKeySize]byte
	seq        uint16
	ack        uint16
}

type ListenConn struct {
	udpconn    *net.UDPConn
	conns      map[string]*Conn
	halfconns  map[string]*HalfConn
	newconns   chan *Packet
	pubkey     [ed25519.PublicKeySize]byte
	privatekey [ed25519.PrivateKeySize]byte
	closed     chan struct{}
	sndch      chan *Packet
}

func Listen(addr string) (ret *ListenConn, err error) {
	udpconn, err := net.Listen("udp", addr)
	if err != nil {
		return
	}
	ret = new(ListenConn)
	ret.pubkey, ret.privatekey, err = ed25519.GenerateKey(rand.Reader)
	ret.conns = make(map[string]*Conn)
	ret.newconns = make(chan *Packet, 5) // Backlog = 5
	go ret.Daemon()
	return ret
}

func (l *ListenConn) Receiver() {
	buff := make([]byte, 4096)
	for {
		n, addr, err := l.udpconn.ReadFrom(buff)
		if err != nil {
			if err == io.EOF || err == io.ErrClosedPipe {
				l.Close()
			}
		}
	}
}

func (l *ListenConn) Dispatcher() {
	for {
		select {
		case <-closed:
			for _, conn := range l.conns {
				conn.Close()
			}
			return
		}
	}
}

func (l *ListenConn) Accept() (conn *Conn, err error) {
	pkt := <-l.newconns

}

func (l *ListenCOnn) Close() (err error) {
	closed <- struct{}{}
}
