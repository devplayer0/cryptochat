# Introduction
CryptoChat is a simple end-to-end encrypted chat application inspired by PictoChat, an application for chatting with
local users on the Nintendo DS series of handheld game consoles. A web interface for sending and receiving messages is
provided.

## Functionality
CryptoChat is a single Go binary which runs on a users machine. They start the program and two HTTP servers are started:
the "UI" and "API" servers. The UI server (by default) binds only to the loopback interface and provides a Vue.js SPA
and accompanying REST API for sending / receiving messages. The API server forms the actual implementation of the
CryptoChat protocol. This server binds on a random port on all interfaces. Peers discover each other via DNS-SD over
mDNS.

## Implementation
### Cryptography
The implementation of end-to-end encryption in CryptoChat is based upon TLS, taking advantage of server _and_ client
certificates to verify identities. Every user has a private key and certificate which are used with TLS to authenticate
and encrypt all peer-to-peer communications. When a user wants to send a message to a new peer, the server will establish
a connection to the peer and allow the user to verify (using an out-of-band verification mechanism comparing both the
certificate's UUID and corresponding fingerprint) that the peer is who they claim to be. Once this step is completed,
the client presents its certificate to the peer's server and they can verify the user. The fingerprint is simply the
SHA-1 hash of the certificate.

### Persistence
CryptoChat implements a public key infrastructure via TLS and a SQLite database. On initial startup, after creating the
database tables, the server generates its RSA private key and X.509 certificate. This will be used to prove the user's
identity (when both making and receiving API calls) and encrypt messages. This certificate and key pair is immediately
persisted to the database. The common name on the certificate is set to a freshly generated UUID (the primary key in the
database). When a new peer is encountered (server or client), their UUID and certificate are stored in the database with
an "unverified" status.

### Verification
Go's `tls` package allows for flexible configuration of both the client and server's certificate verification process.
A generic function, `verifyPeer()` handles all cases for connections to the API server (client or server). If the
certificate belongs to a new user, an appropriate entry is created as described above. Once an unverified entry exists,
future connections will compare the UUID and validate the presented certificate against the stored one. The connection
will continue to block until the UI user confirms the fingerprint.

### Peer-to-Peer REST API
Once all of the verification has taken place, the peer-to-peer API is simple and currently contains only a single
endpoint: `/rooms/{room}/message`. If a client wishes to send a message to all peers in a message room, they need only
use this endpoint, providing a JSON object with the `username` and desired message `content`. The verification layer
described above will take care of all authentication and message encryption. Upon receipt of a request to this API, the
server will push the message to the client.

### UI REST API
The UI REST API is unencrypted (running only on the loopback interface) and allows the client to:
 - Retrieve their UUID and fingerprint (`/api/info`)
 - Verify / unverify a user (`POST` or `DELETE` on `/api/users/{uuid}/verify`)
 - List discovered rooms (`/rooms`)
 - Join / leave a room (`POST` or `DELETE` on `/api/rooms/{room}`)
 - Send a message to all members of a room (`/api/rooms/{room}/message`)

When a verification request is triggered on the server, `verifyPeer()` uses a Server Side Events stream to push the
UUID and fingerprint of the user to the browser for review by the user. The user can then decide whether or not to
use `/api/users/{uuid}/verify` to mark that user as verified.

Messages received by the server are also sent via a different Server Side Events stream for presentation to the user.

### Peer / room discovery
CryptoChat uses DNS-SD for discovering local peers and rooms. On server startup, both a resolver and server are started.
Published service records allow for the discovery of other users (their IP address, API port and UUID) as well as rooms.
Room membership is defined by the `room=` TXT records that a user's mDNS server publishes. When a user wishes to join or
leave a room (using the `/api/rooms/{room}`), the server updates the set of TXT records it publishes.

### Web interface
A very simple and prototype web interface powered by Vue.js is provided, which is served by the UI server and talks to
the UI REST API.
