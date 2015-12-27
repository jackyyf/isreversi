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
	"strings"
	"fmt"
	"net"
	"io"
	"encoding/binary"
)

var exitflag bool = false
var pubkey *[ed25519.PublicKeySize]byte
var privkey *[ed25519.PrivateKeySize]byte
var users map[string]*common.User
var games map[uint16]*common.Game
var rl *readline.Instance
var rid uint16 = 0

var completer = readline.NewPrefixCompleter(
	readline.PcItem("exit"),
	readline.PcItem("kick"),
	readline.PcItem("msg"),
	readline.PcItem("users"),
	readline.PcItem("newgame"),
	readline.PcItem("games"),
	readline.PcItem("watch"),
	readline.PcItem("closegame"),
)

func main() {
	sw.ParseEnv = false
	users = make(map[string]*common.User)
	games = make(map[uint16]*common.Game)
	var err error
	pubkey, privkey, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	socket, err := utp.NewSocket("udp", ":5428")
	if err != nil {
		panic(err)
	}
	rl, err = readline.NewEx(&readline.Config{
		Prompt: "> ",
		// HistoryFile: "./readline.history",
		InterruptPrompt: "Ctrl-C received, exiting ...",
		EOFPrompt: "\n",
		AutoComplete: completer,
	})
	if err != nil {
		panic(err)
	}
	go server(socket)
	// Console part
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
		case line == "exit":
			fmt.Fprintf(rl.Stderr(), "Exiting... Kick all client\n")
			for _, user := range users {
				user.Kick("Server closed")
			}
			return
		case strings.HasPrefix(line, "msg "):
			args, err := sw.Parse(line)
			if err != nil {
				fmt.Fprintln(rl.Stderr(), "Invalid line:", err.Error())
				continue
			}
			args = args[1:]
			if len(args) == 0 {
				fmt.Fprintf(rl.Stderr(), "Usage: msg [user] message\n")
				continue
			}
			if len(args) == 1 { // Broadcast
				for _, user := range users {
					go user.SendMessage("", args[0])
				}
				continue
			}
			if len(args) == 2 { // Send to specific user
				user, ok := users[args[0]]
				if ok {
					user.SendMessage("", args[1])
				} else {
					fmt.Fprintf(rl.Stderr(), "No such user: %s\n", args[0])
				}
			}
		case line == "users":
			fmt.Fprintf(rl.Stderr(), "Online users: \n")
			for name, user := range users {
				fmt.Fprintf(rl.Stderr(), "    %s [%s]: ", name, user.Conn.RemoteAddr().String())
				if user.Game == nil {
					fmt.Fprintf(rl.Stderr(), "Idle\n")
				} else {
					fmt.Fprintf(rl.Stderr(), "In Room %d\n", user.Game.ID)
				}
			}
		case strings.HasPrefix(line, "kick "):
			args, err := sw.Parse(line)
			if err != nil {
				fmt.Fprintln(rl.Stderr(), "Invalid line:", err.Error())
				continue
			}
			args = args[1:]
			if len(args) == 0 {
				fmt.Fprintf(rl.Stderr(), "Usage: kick user [reason]\n")
				continue
			}
			reason := "Kicked with no reason."
			if len(args) > 1 {
				reason = fmt.Sprint("Kicked by server: ", args[1])
			}
			user, ok := users[args[0]]
			if ok {
				user.Kick(reason)
			} else {
				fmt.Fprintf(rl.Stderr(), "No such user: %s\n", args[0])
			}
		case line == "newgame":
			game := common.NewGame()
			game.ID = rid
			rid ++
			games[game.ID] = game
			fmt.Fprintf(rl.Stderr(), "New room #%d opened.\n", game.ID)
		case line == "games":
			fmt.Fprintf(rl.Stderr(), "Current open game: \n")
			for _, game := range games {
				fmt.Fprintf(rl.Stderr(), "    Room #%d: ", game.ID)
				if game.Black != nil {
					fmt.Fprintf(rl.Stderr(), "Black: %s ", game.Black.Name)
				} else {
					fmt.Fprintf(rl.Stderr(), "Black: [free] ")
				}
				if game.White != nil {
					fmt.Fprintf(rl.Stderr(), "White: %s ", game.White.Name)
				} else {
					fmt.Fprintf(rl.Stderr(), "White: [free] ")
				}
				fmt.Fprintln(rl.Stderr())
			}
		case strings.HasPrefix(line, "watch "):
			args, err := sw.Parse(line)
			if err != nil {
				fmt.Fprintln(rl.Stderr(), "Invalid line:", err.Error())
				continue
			}
			args = args[1:]
			if len(args) == 0 {
				fmt.Fprintf(rl.Stderr(), "Usage: watch ID\n")
				continue
			}
			var rid uint16
			if n, err := fmt.Sscanf(args[0], "%d", &rid); n != 1 || err != nil {
				fmt.Fprintf(rl.Stderr(), "Invalid room ID: %s\n", args[0])
				continue
			}
			if room, ok := games[rid]; ok {
				if room.Turn != 0 {
					for i := 0; i < 8; i ++ {
						for j := 0; j < 8; j ++ {
							ch := '.'
							if room.Board[i][j] == 1 {
								ch = 'O'
							} else if room.Board[i][j] == 2 {
								ch = 'X'
							}
							fmt.Fprintf(rl.Stderr(), "%c", ch)
						}
						fmt.Fprintln(rl.Stderr())
					}
					fmt.Fprintln(rl.Stderr())
				}
				if room.Black != nil {
					fmt.Fprintf(rl.Stderr(), "Black: %s ", room.Black.Name)
				} else {
					fmt.Fprintf(rl.Stderr(), "Black: [free] ")
				}
				if room.Turn == 1 {
					fmt.Fprintf(rl.Stderr(), "[Thinking]")
				}
				if room.White != nil {
					fmt.Fprintf(rl.Stderr(), "White: %s ", room.White.Name)
				} else {
					fmt.Fprintf(rl.Stderr(), "White: [free] ")
				}
				if room.Turn == 2 {
					fmt.Fprintf(rl.Stderr(), "[Thinking]")
				}
				fmt.Fprintln(rl.Stderr())
			} else {
				fmt.Fprintf(rl.Stderr(), "Room #%d does not exists!\n", rid)
			}
		case strings.HasPrefix(line, "closegame "):
			args, err := sw.Parse(line)
			if err != nil {
				fmt.Fprintln(rl.Stderr(), "Invalid line:", err.Error())
				continue
			}
			args = args[1:]
			if len(args) == 0 {
				fmt.Fprintf(rl.Stderr(), "Usage: closegame ID\n")
				continue
			}
			var rid uint16
			if n, err := fmt.Sscanf(args[0], "%d", &rid); n != 1 || err != nil {
				fmt.Fprintf(rl.Stderr(), "Invalid room ID: %s\n", args[0])
				continue
			}
			if room, ok := games[rid]; ok {
				if room.Black != nil {
					room.Black.Leave()
				}
				if room.White != nil {
					room.White.Leave()
				}
				delete(games, rid)
			} else {
				fmt.Fprintf(rl.Stderr(), "Room #%d does not exist!\n", rid)
			}
		default:
			fmt.Fprintln(rl.Stderr(), "Commands: ")
			fmt.Fprint(rl.Stderr(), completer.Tree("    "))
		}
	}
}

func server(socket *utp.Socket) {
	var id = 0
	for {
		if (exitflag) {
			break
		}
		conn, err := socket.Accept()
		if err != nil {
			fmt.Fprintln(rl.Stderr(), "Unable to accept:", err)
		}
		go client(conn, id)
		id ++
	}
}

func client(conn net.Conn, id int) {
	user := new(common.User)
	user.Conn = conn
	user.Pubkey = pubkey
	user.Privkey = privkey
	pkt, err := proto.ReadPacket(user.Conn)
	if err != nil {
		fmt.Fprintf(rl.Stderr(), "[Conn %d] Read packet error: %s\n", id, err.Error())
		user.Conn.Close()
		return
	}
	if pkt.Frametype != 0 || len(pkt.Payload) != 64 {
		fmt.Fprintf(rl.Stderr(), "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
		user.Conn.Close()
		return
	}
	user.RPubKey = new([ed25519.PublicKeySize]byte)
	copy(user.RPubKey[:], pkt.Payload[:32])
	if !pkt.Validate(user.RPubKey) {
		fmt.Fprintf(rl.Stderr(), "[Conn %d] Invalid sign for packet!\n", id)
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
		fmt.Fprintf(rl.Stderr(), "[Conn %d] !!WARNING!!, random pool error: %s, using empty challenge string!\n", id, err.Error())
	}
	copy(payload[64:], challenge)
	pkt = proto.NewPacket(1, payload)
	pkt.Sign(privkey)
	_, err = user.Conn.Write(pkt.Bytes())
	if err != nil {
		fmt.Fprintf(rl.Stderr(), "[Conn %d] Write packet error: %s\n", id, err.Error())
		user.Conn.Close()
		return
	}
	// Server validation
	pkt, err = proto.ReadPacket(user.Conn)
	if err != nil {
		fmt.Fprintf(rl.Stderr(), "[Conn %d] Read packet error: %s\n", id, err.Error())
		user.Conn.Close()
		return
	}
	if !pkt.Validate(user.RPubKey) {
		fmt.Fprintf(rl.Stderr(), "[Conn %d] Invalid sign for packet!\n", id)
		user.Conn.Close()
		return
	}
	if pkt.Frametype != 2 || len(pkt.Payload) != 32 {
		fmt.Fprintf(rl.Stderr(), "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
		user.Conn.Close()
		return
	}
	if !bytes.Equal(pkt.Payload, challenge) {
		fmt.Fprintf(rl.Stderr(), "[Conn %d] Invalid challenge data!\n", id)
		user.Conn.Close()
		return
	}
	pkt = proto.NewPacket(3, []byte{})
	pkt.Sign(privkey)
	_, err = user.Conn.Write(pkt.Bytes())
	if err != nil {
		fmt.Fprintf(rl.Stderr(), "[Conn %d] Write packet error: %s\n", id, err.Error())
		user.Conn.Close()
		return
	}
	// Login
	for {
		pkt, err = proto.ReadPacket(user.Conn)
		if err != nil {
			fmt.Fprintf(rl.Stderr(), "[Conn %d] Read packet error: %s\n", id, err.Error())
			user.Conn.Close()
			return
		}
		if !pkt.Validate(user.RPubKey) {
			fmt.Fprintf(rl.Stderr(), "[Conn %d] Invalid sign for packet!\n", id)
			user.Conn.Close()
			return
		}
		if pkt.Frametype != 4 {
			fmt.Fprintf(rl.Stderr(), "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
			user.Conn.Close()
			return
		}
		username, _, err := proto.ReadString(pkt.Payload)
		if err != nil {
			fmt.Fprintf(rl.Stderr(), "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
			user.Conn.Close()
			return
		}
		if _, ok := users[username]; !ok {
			// Username ok
			fmt.Fprintf(rl.Stderr(), "[Conn %d] User %s logged in, from %s\n", id, username, user.Conn.RemoteAddr().String())
			pkt = proto.NewPacket(4, []byte{0})
			pkt.Sign(privkey)
			_, err = user.Conn.Write(pkt.Bytes())
			if err != nil {
				fmt.Fprintf(rl.Stderr(), "[Conn %d] Write packet error: %s\n", id, err.Error())
				user.Conn.Close()
				return
			}
			users[username] = user
			user.Name = username
			defer func(name string) { users[name].Kick(""); delete(users, name) } (username) // Delete on leave
			break
		}
		pkt = proto.NewPacket(4, []byte{1})
		pkt.Sign(privkey)
		_, err = user.Conn.Write(pkt.Bytes())
		if err != nil {
			fmt.Fprintf(rl.Stderr(), "[Conn %d] Write packet error: %s\n", id, err.Error())
			user.Conn.Close()
			return
		}
	}
	for !user.Kicked {
		pkt, err := proto.ReadPacket(user.Conn)
		if err != nil {
			fmt.Fprintf(rl.Stderr(), "[Conn %d] Read packet error: %s\n", id, err.Error())
			user.Conn.Close()
			return
		}
		if !pkt.Validate(user.RPubKey) {
			fmt.Fprintf(rl.Stderr(), "[Conn %d] Invalid sign for packet!\n", id)
			user.Conn.Close()
			return
		}
		buf := make([]byte, 16384)
		switch(pkt.Frametype) {
		case 5: // Quit packet
			reason, _, err := proto.ReadString(pkt.Payload)
			if err != nil {
				fmt.Fprintf(rl.Stderr(), "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
				user.Conn.Close()
				return
			}
			fmt.Fprintf(rl.Stderr(), "[Conn %d] Client Quit: %s\n", id, reason)
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
				fmt.Fprintf(rl.Stderr(), "[Conn %d] Write packet error: %s\n", id, err.Error())
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
				fmt.Fprintf(rl.Stderr(), "[Conn %d] Write packet error: %s\n", id, err.Error())
				user.Conn.Close()
				return
			}
		case 8:
			if len(pkt.Payload) != 2 {
				fmt.Fprintf(rl.Stderr(), "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
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
				fmt.Fprintf(rl.Stderr(), "[Conn %d] Write packet error: %s\n", id, err.Error())
				user.Conn.Close()
				return
			}
		case 9:
			user.Leave()
			pkt = proto.NewPacket(9, []byte{})
			pkt.Sign(privkey)
			_, err = user.Conn.Write(pkt.Bytes())
			if err != nil {
				fmt.Fprintf(rl.Stderr(), "[Conn %d] Write packet error: %s\n", id, err.Error())
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
				fmt.Fprintf(rl.Stderr(), "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
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
				fmt.Fprintf(rl.Stderr(), "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
				user.Conn.Close()
				return
			}
			msg, p, err := proto.ReadString(p)
			if err != nil {
				fmt.Fprintf(rl.Stderr(), "[Conn %d] Unexpected packet type %d\n", id, pkt.Frametype)
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
