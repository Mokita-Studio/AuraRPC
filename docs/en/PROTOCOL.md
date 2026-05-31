# PROTOCOL.md

# IPC Protocol

AuraRPC communicates with the Discord client using its local endpoint.

## Connection

- **Windows:** named pipe `\\.\pipe\discord-ipc-{0..9}`
- **Linux/macOS:** Unix socket `discord-ipc-{0..9}` under `$XDG_RUNTIME_DIR`, `$TMPDIR` or `/tmp`.
- The client attempts to connect sequentially to available pipes from 0 to 9 until a link is established.
- **One connection per process.** Discord serves a single RPC connection, so several cannot be kept open at once. Switching to a preset with a different `client_id` **disconnects** the current connection and **reconnects** with the new id.

## Frame Format

The structure is binary (little-endian):
```text
[ Opcode (4 bytes) ] [ Length (4 bytes) ] [ JSON Payload (N bytes) ]
```
- Main Opcodes: `0` (Handshake), `1` (Frame), `2` (Close), `3` (Ping), `4` (Pong).

## Single Write Rule (Windows)

On Windows, sending the header and payload in separate calls corrupts the synchronous pipe. AuraRPC assembles the full frame in a memory buffer and transmits it via a single `Write` call.

## Data Flow

1. **Handshake:** On connection, it sends opcode `0` with the `client_id` and waits for `READY`.
2. **Set Activity:** To update the status, it sends opcode `1` with the `SET_ACTIVITY` command and the activity JSON. The presence is cleared by **closing** the connection (Disconnect).

## Background reader and reconnection

While the connection is live, a blocking reader goroutine runs that:

- Detects a server-side close **immediately** (EOF when Discord quits or crashes), avoiding the "zombie connected" state without polling (the read parks in the network poller at zero CPU cost).
- Answers `PING` frames with `PONG` and drains `SET_ACTIVITY` acknowledgements.

On connection loss the client reconnects with exponential backoff (1, 2, 4, 8, 16 s, capped at 30 s) and reapplies the last activity. Cancellation closes the pipe at once so the resource is released without leaks.

*Rate-limit note:* Discord throttles the **frequency of new** RPC connections. Switching very rapidly and repeatedly between presets with different `client_id`s can make Discord temporarily delay the `READY`; it recovers after an idle period. This is an external Discord limit, not an AuraRPC one.