# Protocol

## 1. General Packet format

+--------------------+-----------+---------------+------------+----------------+-------------------+
| ed25519 public key | packet ID | packet length | packet ack | packet payload | ed25519 signature |
|      32 bytes      |  2 bytes  |    2 bytes    |   2 bytes  |  <= 410 bytes  |      64 bytes     |
+--------------------+-----------+---------------+------------+----------------+-------------------+

### 1.1 ed25519 key pairs
Ed25519 signatures are elliptic-curve signatures, and it has very small key (32 bytes) and signature size(64 bytes), which is very suitable for UDP transmissions, whose maximum packet size is guarenteed to only 512 bytes at least.

In this project, ed25519 public key is also designed to be used as "Session ID" or "Connection ID". This guarentees the same user will only have at most one connection.

### 1.2 Signatures
Signature part of the packet contains ed25519 signature of following fields: "packet ID", "packet length", "packet ack", "packet payload".
Signatures are designed to protect clients away from MITM attack.
