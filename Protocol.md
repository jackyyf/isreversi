# Protocol

## 1. Design

Protocol is based on uTP, a protocol which is widely used on BitTorrent protocol, and it's standardized by bittorrent [BEP0029](http://www.bittorrent.org/beps/bep_0029.html).

### 1.1 Security: ed25519
Ed25519 signatures are elliptic-curve signatures, and it has very small key (32 bytes) and signature size(64 bytes), and is extremely fast on modern CPUs (fits in L1 cache).
Each frame must be signed using its private key, and validated at the other side.

### 1.2 Frame structure

| Payload length | Frame type | Packet payload | ed25519 signature |
|:--------------:|:----------:|:--------------:|:-----------------:|
|   2 bytes      |  2 bytes   | <= 65535 bytes |      64 bytes     |

Note: payload length DOES NOT include field `Frame Type` and `packet payload`.
Signature of this frame is the ed25519 signature using send side private key, with all 3 fields above.

### 1.3 Type definition

#### 1.3.1 Integers
* All integers are **big endian** encoded.  
* Size for `int8`, `uint8` is 1 byte.
* Size for `int16`, `uint16` is 2 byte.
* Size for `int32`, `uint32` is 4 byte.
* Size for `int64`, `uint64` is 8 byte.
* `int` and `uint` are in variable length, using `varint` encoding specified in [protobuf](https://developers.google.com/protocol-buffers/docs/encoding#varints).
For signed version `int`, refer to [this section](https://developers.google.com/protocol-buffers/docs/encoding#types) of protobuf.

#### 1.3.2 Float points
Only float32 and float64 are supported, which are **big endian** encoded in IEEE754 format.

#### 1.3.3 Strings
Strings are in variable length, which is prefixed by its length using `uint16` type.
All strings **MUST BE** encoded in UTF-8.

#### 1.3.4 Bytes (binary)
Bytes can be in either variable length or fixed length(specified in protocol).
For variable length binary bytes, length in `uint16` type should be added before actually binary data.

#### 1.3.5 Tuple
A tuple of some types is simply concat them together.

## 2. Frame Types

### 2.1 Client Hello
| Frame type | Direction | Client pubkey | Client challenge |
|:----------:|:---------:|:-------------:|:----------------:|
|     0      |  C --> S  |    32 bytes   |     32 bytes     |

This packet is only used during initialize process, which is the first packet client sent to server, give its public key and a random challenge string.

### 2.2 Server Hello
| Frame type | Direction | Server pubkey | Client challenge | Server challenge |
|:----------:|:---------:|:-------------:|:----------------:|:----------------:|
|     1      |  C <-- S  |    32 bytes   |     32 bytes     |     32 bytes     |

This packet is only used during initialize process, which is the first packet server responsed back to client, give its public key, client's chanllenge and server's challenge.

### 2.3 Client Confirm
| Frame type | Direction | Server challenge |
|:----------:|:---------:|:----------------:|
|     2      |  C --> S  |     32 bytes     |

This packet is only used during initialize process, which is the second packet client sent to server, with only server challenge. Since packet is signed, now server is confirmed that client has private key for that public key.

### 2.4 Server Confirm
| Frame type | Direction |
|:----------:|:---------:|
|     3      |  C <-- S  |

This packet is only used during initialize process, which is the second packet server response back to client, to confirm the establishment of the connection.    

### 2.5 Register
| Frame type | Direction | Username |
|:----------:|:---------:|:--------:|
|     4      |  C --> S  |  string  |

| Frame type | Direction | Result |
|:----------:|:---------:|:------:|
|     4      |  C <-- S  |  uint8 |

This packet is used for to register after the establishment of the connection.  
For client to server, the option in payload is username that client is willing to use.  
For server to client, the only payload is result, 0 means accepted and can continue, where 1 means rejected, and client should try another username, using same packet.

### 2.6 Quit / Kick
| Frame type | Direction | Reason |
|:----------:|:---------:|:------:|
|     5      |  C <-> S  | string |

This packet is used for either quit request by client, or kick notification for server.  
After sending the packet, they should wait at least 1 second to close the connection.

### 2.7 Room list
| Frame type | Direction |
|:----------:|:---------:|
|     6      |  C --> S  |

| Frame type | Direction |   Data   |
|:----------:|:---------:|:--------:|
|     6      |  C <-- S  |   bytes  |

This packet is used to obtain room information.  
Client send a packet with no payload to request data from server.  
Server should response back with same frame type, and contains data for room list.  

Data is in a tuple of tuple(uint16, string, string).  
For each tuple, the corresponding field are Room ID, Player A, Player B.

### 2.8 Player list
| Frame type | Direction |
|:----------:|:---------:|
|     7      |  C --> S  |

| Frame type | Direction |   Data   |
|:----------:|:---------:|:--------:|
|     7      |  C <-- S  |   bytes  |

This packet is used to obtain player list.  
Client send a packet with no payload to request data from server.  
Server should response back with same frame type, and contains data for player list.

Data is in a tuple of string, which contains Player Name.

### 2.9 Join Game
| Frame type | Direction | Room ID |
|:----------:|:---------:|:-------:|
|     8      |  C --> S  | uint16  |

| Frame type | Direction | Result |
|:----------:|:---------:|:------:|
|     8      |  C <-- S  | uint8  |

This packet is used to join a game.  
Client send a packet with room ID to server.  
Server should response back with same frame type, and contains result for previous action.  

Result list:  
0: Join OK, you are now inside game you are willing to join.  
1: Join failed, room does not exists.  
2: Join failed, room is full.  
3: Join failed, you are in another room.

### 2.10 Leave game
| Frame type | Direction |
|:----------:|:---------:|
|     9      |  C <-> S  |

This packet is used to leave a room.  
Client send a packet to server, and server should response back with exactly same packet, since the only reason that will cause leaving to fail is he is not in the room.  
For invalid request, server may use other action like send a chat message, or disconnect the client.

### 2.11 Game restart
| Frame type | Direction |
|:----------:|:---------:|
|     10     |  C --> S  |

This packet is used to restart a game. Game can only be restarted at end state (one side has win the game, or a tie).  
Client send a packet to server, to indicate update its ready state.  
For invalid request, server may use other action like send a chat message, or disconnect the client.

### 2.12 Place
| Frame type | Direction | Position |
|:----------:|:---------:|:--------:|
|     11     |  C --> S  |   uint8  |

This packet is used to place a chess on board, the high 4 bit is x and low 4 bit is y.  
Server doesn't response back, for a valid request, server should send "Board Update" instead.
For invalid request, server may use other action like send a chat message, or disconnect the client.

### 2.13 Room Update
| Frame type | Direction |  Placed  |  Color  |  Turn  |  White  |  Black  |
|:----------:|:---------:|:--------:|:-------:|:------:|:-------:|:-------:|
|     12     |  C <-- S  |  uint64  |  uint64 |  uint8 |  string |  string |

This packet is update the current state of board. Since the board is 8x8, each bit of the first uint64 "Placed" is used to set if a point is placed, and the second uint64 set its color, black = 0, white = 1. Turn means who should place now, 0 = game over, 1 = black, 2 = white.

### 2.14 Chat message
| Frame type | Direction | Player | Message |
|:----------:|:---------:|:------:|:-------:|
|     13     |  C <-> S  | string |  string |

This packet is used to send and receive chat message.  
For client sent packet, player is the name he wants to send to, or a empty string if he wants to broadcast.  
For server sent packet, player is the name who sends the message, or a empty string indicates it's server's notification.
