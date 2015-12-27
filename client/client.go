package main

import (
	"github.com/jackyyf/isreversi/proto"
	"github.com/jackyyf/isreversi/utp"
	"github.com/jackyyf/isreversi/common"
	"github.com/agl/ed25519"
	sw "github.com/mattn/go-shellwords"
	"gopkg.in/readline.v1"
	"crypto/rand"
	"bytes"
	"os"
	"fmt"
	"strconv"
	"net"
	"strings"
	"io"
	"encoding/binary"
	"time"
)

var exitflag bool = false
var pubkey *[ed25519.PublicKeySize]byte
var privkey *[ed25519.PrivateKeySize]byte
var user *common.User
var game *common.Game
var rid uint16 = 0
var port uint16
var rl *readline.Instance
var stderr io.Writer

var completer = readline.NewPrefixCompleter(
	readline.PcItem("login"),
	readline.PcItem("games"),
	readline.PcItem("users"),
	readline.PcItem("join"),
	readline.PcItem("place"),
	readline.PcItem("restart"),
	readline.PcItem("leave"),
	readline.PcItem("exit"),
)

func main() {
	user = new(common.User)
	game = new(common.Game)
	var err error
	pubkey, privkey, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	socket, err := utp.Dial("localhost:5428")
	if err != nil {
		panic(err)
	}
	if !handshake(socket) {
		panic("Handshake to server failed!")
	}
	rl, err = readline.NewEx(&readline.Config{
		Prompt: "> ",
		// HistoryFile: "./readline.history",
		InterruptPrompt: "Ctrl-C received, exiting ...",
		EOFPrompt: "\n",
		AutoComplete: completer,
	})
	stderr = rl.Stderr()
	go downstream()
	upstream()
}

func upstream() {
	buff := make([]byte, 16384)
	for {
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			line = "exit"
		}
		if err == io.EOF {
			continue
		}
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "login "):
			// arg1 => username
			args, err := sw.Parse(line)
			if err != nil {
				fmt.Fprintln(stderr, "Invalid line:", err.Error())
				continue
			}
			args = args[1:]
			if len(args) == 0 {
				fmt.Fprintf(stderr, "Usage: login username\n")
				continue
			}
			if len(args) == 1 {
				proto.WriteString(buff, args[0])
				pkt := proto.NewPacket(4, buff[:len(args[0]) + 2])
				pkt.Sign(privkey)
				_, err = user.Conn.Write(pkt.Bytes())
				if err != nil {
					fmt.Fprintf(stderr, "Write packet error: %s\n", err.Error())
					panic("Socket error!")
				}
			}
		case strings.HasPrefix(line, "exit"):
			// arg1 => reason
			reason := "Disconnected"
			args, err := sw.Parse(line)
			if err != nil {
				fmt.Fprintln(stderr, "Invalid line:", err.Error())
				continue
			}
			if len(args) >= 2 {
				reason = args[1]
			}
			proto.WriteString(buff, reason)
			pkt := proto.NewPacket(5, buff[:len(reason) + 2])
			pkt.Sign(privkey)
			_, err = user.Conn.Write(pkt.Bytes())
			if err != nil {
				fmt.Fprintf(stderr, "Write packet error: %s\n", err.Error())
				panic("Socket error!")
			}
			time.Sleep(500 * time.Millisecond)
			os.Exit(0)
		case line == "games":
			pkt := proto.NewPacket(6, []byte{})
			pkt.Sign(privkey)
			_, err = user.Conn.Write(pkt.Bytes())
			if err != nil {
				fmt.Fprintf(stderr, "Write packet error: %s\n", err.Error())
				panic("Socket error!")
			}
		case line == "players":
			pkt := proto.NewPacket(7, []byte{})
			pkt.Sign(privkey)
			_, err = user.Conn.Write(pkt.Bytes())
			if err != nil {
				fmt.Fprintf(stderr, "Write packet error: %s\n", err.Error())
				panic("Socket error!")
			}
		case strings.HasPrefix(line, "join "):
			// arg1 => roomid

			args, err := sw.Parse(line)
			if err != nil {
				fmt.Fprintln(stderr, "Invalid line:", err.Error())
				continue
			}
			args = args[1:]
			if len(args) == 0 {
				fmt.Fprintf(stderr, "Usage: join roomid\n")
				continue
			}
			if len(args) == 1 {
				r, err := strconv.Atoi(args[0])
				if err != nil {
					fmt.Fprintf(stderr, "Invalid roomid %s\n", args[0])
				}
				roomid := uint16(r)
				binary.BigEndian.PutUint16(buff, roomid)
				pkt := proto.NewPacket(8, buff[:2])
				pkt.Sign(privkey)
				_, err = user.Conn.Write(pkt.Bytes())
				if err != nil {
					fmt.Fprintf(stderr, "Write packet error: %s\n", err.Error())
					panic("Socket error!")
				}
			}
		case line == "leave":
			pkt := proto.NewPacket(9, []byte{})
			pkt.Sign(privkey)
			_, err = user.Conn.Write(pkt.Bytes())
			if err != nil {
				fmt.Fprintf(stderr, "Write packet error: %s\n", err.Error())
				panic("Socket error!")
				return
			}
		case line == "restart":
			pkt := proto.NewPacket(10, []byte{})
			pkt.Sign(privkey)
			_, err = user.Conn.Write(pkt.Bytes())
			if err != nil {
				fmt.Fprintf(stderr, "Write packet error: %s\n", err.Error())
				panic("Socket error!")
			}
		case strings.HasPrefix(line, "place "):
			args, err := sw.Parse(line)
			if err != nil {
				fmt.Fprintln(stderr, "Invalid line:", err.Error())
				continue
			}
			args = args[1:]
			if len(args) < 2 {
				fmt.Fprintf(stderr, "Usage: place x y\n")
				continue
			}
			// arg1 => x, arg2 => y
			x, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Fprintf(stderr, "Invalid number %s\n", args[0])
			}
			y, err := strconv.Atoi(args[1])
			if err != nil {
				fmt.Fprintf(stderr, "Invalid number %s\n", args[1])
			}
			buff[0] = byte((x << 4) | y)
			pkt := proto.NewPacket(11, buff[:1])
			pkt.Sign(privkey)
			_, err = user.Conn.Write(pkt.Bytes())
			if err != nil {
				fmt.Fprintf(stderr, "Write packet error: %s\n", err.Error())
				panic("Socket error!")
			}
		case strings.HasPrefix(line, "msg "):
			args, err := sw.Parse(line)
			if err != nil {
				fmt.Fprintln(stderr, "Invalid line:", err.Error())
				continue
			}
			args = args[1:]
			if len(args) < 2 {
				fmt.Fprintf(stderr, "Usage: msg player message\n")
				continue
			}
			b := buff
			b, _ = proto.WriteString(b, args[0])
			b, _ = proto.WriteString(b, args[1])
			pkt := proto.NewPacket(13, buff[:len(args[0]) + len(args[1]) + 4])
			pkt.Sign(privkey)
			_, err = user.Conn.Write(pkt.Bytes())
			if err != nil {
				fmt.Fprintf(stderr, "Write packet error: %s\n", err.Error())
				panic("Socket error!")
			}
		default:
			fmt.Fprintln(stderr, "Commands: ")
			fmt.Fprint(stderr, completer.Tree("    "))
		}
	}
}

func downstream() {
	// Downstream
	for {
		pkt, err := proto.ReadPacket(user.Conn)
		if err != nil {
			fmt.Fprintf(stderr, "Read packet error: %s\n", err.Error())
			user.Conn.Close()
			panic("Socket error!")
		}
		if !pkt.Validate(user.RPubKey) {
			fmt.Fprintf(stderr, "Invalid sign for packet!\n")
			user.Conn.Close()
			panic("Socket error!")
		}
		switch(pkt.Frametype) {
		case 4:
			if len(pkt.Payload) != 1 {
				fmt.Fprintf(stderr, "Invalid packet\n")
				user.Conn.Close()
				panic("Socket error!")
			}
			if pkt.Payload[0] != 0 {
				fmt.Fprintf(stderr, "Login failed, please try again.")
			}
		case 5:
			str, _, err := proto.ReadString(pkt.Payload)
			if err != nil {
				fmt.Fprintf(stderr, "Invalid packet\n")
				user.Conn.Close()
				panic("Socket error!")
			}
			fmt.Fprintf(stderr, "You are kicked by server: %s\n", str)
			os.Exit(0)
		case 6:
			fmt.Fprintf(stderr, "Room list: \n")
			b := pkt.Payload
			for len(b) >= 6 {
				uid := binary.BigEndian.Uint16(b)
				b = b[2:]
				ua := ""
				ub := ""
				ua, b, err = proto.ReadString(b)
				if err != nil {
					fmt.Fprintf(stderr, "Invalid packet\n")
					user.Conn.Close()
					panic("Socket error!")
				}
				ub, b, err = proto.ReadString(b)
				if err != nil {
					fmt.Fprintf(stderr, "Invalid packet\n")
					user.Conn.Close()
					panic("Socket error!")
				}
				if ua == "" {
					ua = "[Empty]"
				}
				if ub == "" {
					ub = "[Empty]"
				}
				fmt.Fprintf(stderr, "  Room #%d Black: %s White: %s\n", uid, ua, ub)
			}
		case 7:
			fmt.Fprintf(stderr, "User list: \n")
			b := pkt.Payload
			for len(b) >= 2 {
				ua := ""
				ua, b, err = proto.ReadString(b)
				if err != nil {
					fmt.Fprintf(stderr, "Invalid packet\n")
					user.Conn.Close()
					panic("Socket error!")
				}
				fmt.Fprintf(stderr, "  %s\n", ua)
			}
		case 8:
			if len(pkt.Payload) != 1 {
				fmt.Fprintf(stderr, "Invalid packet\n")
				user.Conn.Close()
				panic("Socket error!")
			}
			switch(pkt.Payload[0]) {
			case 0:
				fmt.Fprintf(stderr, "Join successfully!\n")
			case 1:
				fmt.Fprintf(stderr, "No such room!\n")
			case 2:
				fmt.Fprintf(stderr, "No vacancy in room!\n")
			case 3:
				fmt.Fprintf(stderr, "You are already in a game room!\n")
			}
		case 9:
			fmt.Fprintf(stderr, "You are now in the lobby.\n")
		case 12:
			b := pkt.Payload
			if len(pkt.Payload) <= 21 {
				fmt.Fprintf(stderr, "Invalid packet\n")
				user.Conn.Close()
				panic("Socket error!")
			}
			placed := binary.BigEndian.Uint64(b)
			color := binary.BigEndian.Uint64(b[8:])
			turn := uint8(b[16])
			b = b[17:]
			white, b, err := proto.ReadString(b)
			if err != nil {
				fmt.Fprintf(stderr, "Invalid packet\n")
				user.Conn.Close()
				panic("Socket error!")
			}
			black, b, err := proto.ReadString(b)
			if err != nil {
				fmt.Fprintf(stderr, "Invalid packet\n")
				user.Conn.Close()
				panic("Socket error!")
			}
			fmt.Fprintln(rl.Stdout())
			wc := 0
			bc := 0
			var buff [8]byte
			for i := 0; i <= 63; i ++ {
				buff[i & 7] = '.'
				if (placed & (uint64(1) << uint(i)) != 0) {
					if (color & (uint64(1) << uint(i)) == 0) {
						bc ++
						buff[i & 7] = 'b'
					} else {
						buff[i & 7] = 'w'
						wc ++
					}
				}
				if (i & 7) == 7 {
					fmt.Fprintln(rl.Stdout(), string(buff[:]))
				}
			}
			if white == "" {
				white = "[Empty]"
			}
			if black == "" {
				black = "[Empty]"
			}
			fmt.Fprintf(rl.Stdout(), "White: %s[%d], Black: %s[%d]\n", white, wc, black, bc)
			if turn == 0 {
				fmt.Fprintf(rl.Stdout(), "Game over! ")
				if wc == 0 && bc == 0 {

				} else if wc > bc {
					fmt.Fprintf(rl.Stdout(), "White wins!")
				} else if wc < bc {
					fmt.Fprintf(rl.Stdout(), "Black wins!")
				} else if wc == bc {
					fmt.Fprintf(rl.Stdout(), "Tie")
				}
			} else if turn == 1 {
				fmt.Fprintf(stderr, "Black's turn!\n")
			} else if turn == 2 {
				fmt.Fprintf(stderr, "White's turn!\n")
			}
			fmt.Fprintln(rl.Stdout())
		case 13:
			b := pkt.Payload
			who, b, err := proto.ReadString(b)
			if err != nil {
				fmt.Fprintf(stderr, "Invalid packet\n")
				user.Conn.Close()
				panic("Socket error!")
			}
			msg, b, err := proto.ReadString(b)
			if err != nil {
				fmt.Fprintf(stderr, "Invalid packet\n")
				user.Conn.Close()
				panic("Socket error!")
			}
			if who == "" {
				fmt.Fprintf(stderr, "[SYSTEM] ")
			} else {
				fmt.Fprintf(stderr, "%s: ", who)
			}
			fmt.Fprintln(stderr, msg)
		}
	}
}

func handshake(conn net.Conn) bool {
	var err error
	user.Conn = conn
	user.Pubkey = pubkey
	user.Privkey = privkey
	buf := make([]byte, 64)
	challenge := make([]byte, 32)
	rand.Read(challenge)
	copy(buf, (*user.Pubkey)[:])
	copy(buf[32:], challenge)
	pkt := proto.NewPacket(0, buf)
	pkt.Sign(privkey)
	_, err = user.Conn.Write(pkt.Bytes())
	if err != nil {
		fmt.Fprintf(stderr, "Write packet error: %s\n", err.Error())
		user.Conn.Close()
		return false
	}
	pkt, err = proto.ReadPacket(user.Conn)
	if err != nil {
		fmt.Fprintf(stderr, "Read packet error: %s\n", err.Error())
		user.Conn.Close()
		return false
	}
	if pkt.Frametype != 1 || len(pkt.Payload) != 96 {
		fmt.Fprintf(stderr, "Unexpected packet type %d\n", pkt.Frametype)
		user.Conn.Close()
		return false
	}
	user.RPubKey = new([ed25519.PublicKeySize]byte)
	copy((*user.RPubKey)[:], pkt.Payload[:32])
	if !pkt.Validate(user.RPubKey) {
		fmt.Fprintf(stderr, "Invalid sign for packet!\n")
		user.Conn.Close()
		return false
	}
	if !bytes.Equal(challenge, pkt.Payload[32:64]) {
		fmt.Fprintf(stderr, "Invalid challenge data!\n")
		user.Conn.Close()
		return false
	}
	pkt = proto.NewPacket(2, pkt.Payload[64:])
	pkt.Sign(privkey)
	_, err = user.Conn.Write(pkt.Bytes())
	if err != nil {
		fmt.Fprintf(stderr, "Write packet error: %s\n", err.Error())
		user.Conn.Close()
		return false
	}
	pkt, err = proto.ReadPacket(user.Conn)
	if err != nil {
		fmt.Fprintf(stderr, "Read packet error: %s\n", err.Error())
		user.Conn.Close()
		return false
	}
	if pkt.Frametype != 3 {
		fmt.Fprintf(stderr, "Unexpected packet type %d\n", pkt.Frametype)
		user.Conn.Close()
		return false
	}
	if !pkt.Validate(user.RPubKey) {
		fmt.Fprintf(stderr, "Invalid sign for packet!\n")
		user.Conn.Close()
		return false
	}
	return true
}

/*
func client(conn net.Conn) {
	user.Conn = conn
	user.Pubkey = pubkey
	user.Privkey = privkey
	pkt, err := proto.ReadPacket(user.Conn)
	if err != nil {
		fmt.Fprintf(stderr, "[Conn %d] Read packet error: %s\n", id, err.Error())
		user.Conn.Close()
		return
	}
	if pkt.Frametype != 0 || len(pkt.Payload) != 64 {
		fmt.Fprintf(stderr, "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
		user.Conn.Close()
		return
	}
	user.RPubKey = new([ed25519.PublicKeySize]byte)
	copy(user.RPubKey[:], pkt.Payload[:32])
	if !pkt.Validate(user.RPubKey) {
		fmt.Fprintf(stderr, "[Conn %d] Invalid sign for packet!\n", id)
		user.Conn.Close()
		return
	}
	// Client key & challenge
	payload := make([]byte, 96)
	copy(payload, pubkey[:])
	copy(payload[32:], pkt.Payload[32:])
	challenge := make([]byte, 32)
	_, err = rand.Read(challenge)
	if err != nil {
		fmt.Fprintf(stderr, "[Conn %d] !!WARNING!!, random pool error: %s, using empty challenge string!\n", id, err.Error())
	}
	copy(payload[64:], challenge)
	pkt = proto.NewPacket(1, payload)
	pkt.Sign(privkey)
	_, err = user.Conn.Write(pkt.Bytes())
	if err != nil {
		fmt.Fprintf(stderr, "[Conn %d] Write packet error: %s\n", id, err.Error())
		user.Conn.Close()
		return
	}
	// Server validation
	pkt, err = proto.ReadPacket(user.Conn)
	if err != nil {
		fmt.Fprintf(stderr, "[Conn %d] Read packet error: %s\n", id, err.Error())
		user.Conn.Close()
		return
	}
	if !pkt.Validate(user.RPubKey) {
		fmt.Fprintf(stderr, "[Conn %d] Invalid sign for packet!\n", id)
		user.Conn.Close()
		return
	}
	if pkt.Frametype != 2 || len(pkt.Payload) != 32 {
		fmt.Fprintf(stderr, "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
		user.Conn.Close()
		return
	}
	if !bytes.Equal(pkt.Payload, challenge) {
		fmt.Fprintf(stderr, "[Conn %d] Invalid challenge data!\n", id)
		user.Conn.Close()
		return
	}
	pkt = proto.NewPacket(3, []byte{})
	pkt.Sign(privkey)
	_, err = user.Conn.Write(pkt.Bytes())
	if err != nil {
		fmt.Fprintf(stderr, "[Conn %d] Write packet error: %s\n", id, err.Error())
		user.Conn.Close()
		return
	}
	// Login
	for {
		pkt, err = proto.ReadPacket(user.Conn)
		if err != nil {
			fmt.Fprintf(stderr, "[Conn %d] Read packet error: %s\n", id, err.Error())
			user.Conn.Close()
			return
		}
		if !pkt.Validate(user.RPubKey) {
			fmt.Fprintf(stderr, "[Conn %d] Invalid sign for packet!\n", id)
			user.Conn.Close()
			return
		}
		if pkt.Frametype != 4 {
			fmt.Fprintf(stderr, "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
			user.Conn.Close()
			return
		}
		username, _, err = proto.ReadString(pkt.Payload)
		if err != nil {
			fmt.Fprintf(stderr, "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
			user.Conn.Close()
			return
		}
		if _, ok := users[username]; !ok {
			// Username ok
			fmt.Fprintf(stderr, "[Conn %d] User %s logged in, from %s\n", username, user.Conn.RemoteAddr().String())
			pkt = proto.NewPacket(4, []byte{0})
			pkt.Sign(privkey)
			_, err = user.Conn.Write(pkt.Bytes())
			if err != nil {
				fmt.Fprintf(stderr, "[Conn %d] Write packet error: %s\n", id, err.Error())
				user.Conn.Close()
				return
			}
			users[username] = user
			defer func(name string) { delete(users, name) } (username) // Delete on leave
			break
		}
		pkt = proto.NewPacket(4, []byte{1})
		pkt.Sign(privkey)
		_, err = user.Conn.Write(pkt.Bytes())
		if err != nil {
			fmt.Fprintf(stderr, "[Conn %d] Write packet error: %s\n", id, err.Error())
			user.Conn.Close()
			return
		}
	}
	for !user.Kicked {
		pkt, err := proto.ReadPacket(user.Conn)
		if err != nil {
			fmt.Fprintf(stderr, "[Conn %d] Read packet error: %s\n", id, err.Error())
			user.Conn.Close()
			return
		}
		if !pkt.Validate(user.RPubKey) {
			fmt.Fprintf(stderr, "[Conn %d] Invalid sign for packet!\n", id)
			user.Conn.Close()
			return
		}
		buf := make([]byte, 16384)
		switch(pkt.Frametype) {
		case 5: // Quit packet
			reason, _, err = proto.ReadString(pkt.Payload)
			if err != nil {
				fmt.Fprintf(stderr, "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
				user.Conn.Close()
				return
			}
			fmt.Fprintf(stderr, "[Conn %d] Client Quit: %s\n", id, reason)
			user.Conn.Close()
			return
		case 6: // Roomlist request
			b := buf
			for rid, room := range games {
				if room == nil {
					continue
				}
				usera := ""
				if room.Black != nil {
					usera = room.Black.Name
				}
				userb := ""
				if room.White != nil {
					userb = room.White.Name
				}
				binary.BigEndian.PutUint16(b, rid)
				b = b[2:]
				b, _ = proto.WriteString(b, usera)
				b, _ = proto.WriteString(b, userb)
			}
			length := 16384 - len(b)
			b = buf[:length]
			pkt = proto.NewPacket(6, b)
			pkt.Sign(privkey)
			_, err = user.Conn.Write(pkt.Bytes())
			if err != nil {
				fmt.Fprintf(stderr, "[Conn %d] Write packet error: %s\n", id, err.Error())
				user.Conn.Close()
				return
			}
		case 7: // Player list request
			b := buf
			for username := range users{
				b, _ = proto.WriteString(b, username)
			}
			length := 16384 - len(b)
			b = buf[:length]
			pkt = proto.NewPacket(7, b)
			pkt.Sign(privkey)
			_, err = user.Conn.Write(pkt.Bytes())
			if err != nil {
				fmt.Fprintf(stderr, "[Conn %d] Write packet error: %s\n", id, err.Error())
				user.Conn.Close()
				return
			}
		case 8:
			if len(pkt.Payload) != 2 {
				fmt.Fprintf(stderr, "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
				user.Conn.Close()
				return
			}
			jrid := binary.BigEndian.Uint16(pkt.Payload)
			room := games[jrid]
			result := user.Join(room)
			pkt = proto.NewPacket(8, []byte{result})
			pkt.Sign(privkey)
			_, err = user.Conn.Write(pkt.Bytes())
			if err != nil {
				fmt.Fprintf(stderr, "[Conn %d] Write packet error: %s\n", id, err.Error())
				user.Conn.Close()
				return
			}
		case 9:
			user.Leave()
			pkt = proto.NewPacket(9, []byte{})
			pkt.Sign(privkey)
			_, err = user.Conn.Write(pkt.Bytes())
			if err != nil {
				fmt.Fprintf(stderr, "[Conn %d] Write packet error: %s\n", id, err.Error())
				user.Conn.Close()
				return
			}
		case 10:
			user.Restart()
			if user.Game.Black != nil {
				user.Game.Black.SendMessage("", fmt.Sprintf("%s is now ready.", user.Name))
			}
			if user.Game.White != nil {
				user.Game.White.SendMessage("", fmt.Sprintf("%s is now ready.", user.Name))
			}
		case 11:
			if len(pkt.Payload) != 1 {
				fmt.Fprintf(stderr, "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
				user.Conn.Close()
				return
			}
			xy := uint8(pkt.Payload[0])
			x := xy >> 4
			y := xy & 15
			if user.Game.Black == user {
				if !user.Game.Place(x, y, 1) {
					user.SendMessage("", "Invalid operation")
				}
			} else {
				if !user.Game.Place(x, y, 2) {
					user.SendMessage("", "Invalid operation")
				}
			}
		case 13:
			p := pkt.Payload
			dest, p, err := proto.ReadString(p)
			if err != nil {
				fmt.Fprintf(stderr, "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
				user.Conn.Close()
				return
			}
			msg, p, err := proto.ReadString(p)
			if err != nil {
				fmt.Fprintf(stderr, "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
				user.Conn.Close()
				return
			}
			if dest != "" {
				u, ok := users[dest]
				if !ok {
					user.SendMessage("", "No such user.")
				} else {
					go u.SendMessage(user.Name, msg)
				}
			}
		}
	}
}
*/
