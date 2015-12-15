package socket

import (
	"net"
	"time"
)

type Packet struct {
	body []byte
}

type Conn struct {
	ch    chan *Packet
	timer *time.Timer
}

type ListenConn struct {
	conn *net.UDPConn
}
