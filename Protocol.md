# Protocol

## 1. General Packet format

	+--------------------+-----------+----------------+--------------+------------+----------------+-------------------+
	| ed25519 public key | packet ID | payload length | packet flags | packet ack | packet payload | ed25519 signature |
	|      32 bytes      |  2 bytes  |     10 bits    |    6 bits    |   2 bytes  |  <= 410 bytes  |      64 bytes     |
	+--------------------+-----------+----------------+--------------+------------+----------------+-------------------+

### 1.1 ed25519 key pairs
Ed25519 signatures are elliptic-curve signatures, and it has very small key (32 bytes) and signature size(64 bytes), which is very suitable for UDP transmissions, whose maximum packet size is guarenteed to only 512 bytes at least.

In this project, ed25519 public key is also designed to be used as "Session ID" or "Connection ID". This guarentees the same user will only have at most one connection.

### 1.2 Flags
	+-+-+-+-+-+-+
	|5|4|3|2|1|0|
	+-+-+-+-+-+-+
	|I|S|C|E|R|T|
	|N|V|L|S|E|E|
	|I|H|H|T|C|R|
	|T|S|S|B|V|M|
	+-+-+-+-+-+-+

Bit 5: INIT bit, indicates this is an connection initialize packet. Payload length MUST be 0, with no payload.  
Bit 4: SVHS bit, indicates server has received an INIT packet, and is now willing to establish a connection.  
Bit 3: CLHS bit, indicates client has received an SVHS packet, and responses according to challenge.  
Bit 2: ESTB bit, indicates server has received an CLHS packet, and responses according to challenge.  
Bit 1: RECV bit, indicates one side has received a packet, and last in-order packet ID is in packet ack.  
Bit 0: TERM bit, indicates one side has terminated the connection.  

### 1.4 Signatures
Signature part of the packet contains ed25519 signature of following fields: "packet ID", "packet length", "packet ack", "packet payload".  
Signatures are designed to protect clients away from MITM attack.  
A packet with invalid signature should be dropped sliently.

### 1.5 Error handling

#### 1.5.1 Generic definition
Both sides MUST hold a window of exactly 5 packet, and the ACK number is the last received in-order packet ID.  
Both sides SHOULD initialize a retransmission after 2 duplicate ACKs.  
Both sides SHOULD initialize a retransmission 2, 4, 8, 16 seconds after no ACK from the other side. No ACK from the other side after 32 seconds, the connection SHOULD BE closed.  
Duplicate packets (same packet ID) MUST be dropped and ignored.

#### 1.5.2 Duplicate ACKs received
When 2 duplicate ACKs are received, sender side SHOULD start retransmission from packet ID = ACK + 1.  
After initialization of the retransmission, duplicate ACKs SHOULD be dropped, and handled by timeout handler.

### 1.5.3 No ACK after specific time
When specific time passed and no ACK are received, sender side SHOULD retransmissit ONLY packet LAST\_ACK + 1. After an ACK was received properly (ACK = LAST\_ACK + 1), now sender side MAY expand send window to 5 packet.  

## 2. Protocol

### 2.1 Handshake

Handshake has 4 steps:
	C -> S: A packet with random initial packet ID, and with INIT bit set. ACK field MUST be 0.
	S -> C: A packet with another initial packet ID, and with SVHS bit set. ACK field MUST be client's packet ID. Payload contains a 16 bytes long random binary string.
	C -> S: Response a packet with CLHS and RECV bit set. ACK field MUST be server's packet ID. Payload contains 32 bytes: server's 16 byte payload, and another 16 bytes long random binary string.
	(Client starts its error handling process)
	S -> C: Response a packet with ESTB and RECV bit set. ACK field MUST be client's packet ID. Payload contains 16 bytes: client's 16 byte random string.
	(Server starts its error handling process)
	(Connection established)

In order to prevent similar "SYN Flood attack" in TCP, server MAY hide "cookie" in the SVHS packet.  

### 2.2 Termination

Termination has 3 steps:
	A -> B: Send a packet with TERM bit set.
	B -> A: Send a packet with TERM and RECV bit set.
	A -> B: Send a packet with RECV bit set.
