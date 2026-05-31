// Package discord implements the Rich Presence IPC client: handshake,
// SET_ACTIVITY and reconnect with backoff. Transport is named pipes on
// Windows and Unix sockets elsewhere, with no external dependencies.
package discord

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	opHandshake uint32 = 0
	opFrame     uint32 = 1
	opClose     uint32 = 2
	opPing      uint32 = 3
	opPong      uint32 = 4

	protoVersion = 1
	maxFrameSize = 64 * 1024
)

// ErrPipeNotFound is returned when none of discord-ipc-{0..9} is available.
var ErrPipeNotFound = errors.New("discord: ipc pipe not found")

// State is the IPC connection state.
type State int

const (
	// StateDisconnected: no active connection.
	StateDisconnected State = iota
	// StateConnecting: dialing or handshaking.
	StateConnecting
	// StateConnected: handshake completed.
	StateConnected
)

// String returns the lowercase state name.
func (s State) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	}
	return fmt.Sprintf("state(%d)", int(s))
}

// Client is a Discord Rich Presence IPC client.
//
// Lifecycle: New, then Run in a goroutine. SetActivity is goroutine-safe;
// the last value is replayed on reconnect.
type Client struct {
	clientID string
	pid      int

	mu          sync.Mutex
	state       State
	activity    *Activity
	activitySet bool

	nonce atomic.Int64
	sig   chan struct{}

	// writeMu serializes frame writes so the serve loop (SET_ACTIVITY)
	// and the reader's PONG replies never interleave bytes on the pipe.
	writeMu sync.Mutex
}

// New builds a Client for the given Application/Client ID.
func New(clientID string) *Client {
	return &Client{
		clientID: clientID,
		pid:      os.Getpid(),
		sig:      make(chan struct{}, 1),
	}
}

// State returns the current connection state.
func (c *Client) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// SetActivity stores a as the current activity and schedules a send on
// the next loop tick. Persists across reconnects.
func (c *Client) SetActivity(a Activity) {
	c.mu.Lock()
	cp := a
	c.activity = &cp
	c.activitySet = true
	c.mu.Unlock()
	c.notify()
}

func (c *Client) notify() {
	select {
	case c.sig <- struct{}{}:
	default:
	}
}

// Run keeps the connect -> handshake -> serve -> reconnect loop alive
// until ctx is cancelled.
func (c *Client) Run(ctx context.Context) error {
	defer c.emit(StateDisconnected)
	log.Printf("discord.Run: start client_id=%s pid=%d", c.clientID, c.pid)
	defer log.Printf("discord.Run: exit client_id=%s", c.clientID)

	attempt := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.emit(StateConnecting)

		conn, err := dial()
		if err != nil {
			log.Printf("discord.Run: dial error: %v", err)
			c.emit(StateDisconnected)
			if waitErr := sleep(ctx, backoff(attempt)); waitErr != nil {
				return waitErr
			}
			attempt++
			continue
		}
		log.Printf("discord.Run: pipe opened")

		// Close conn the moment ctx is cancelled so a blocked handshake or
		// read returns at once and the pipe is released. Without this, a
		// client cancelled mid-handshake (which happens when presets are
		// switched faster than they connect) would leak its goroutine and
		// hold a pipe until Discord answered — exhausting the 10 pipes.
		stopWatch := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				_ = conn.Close()
			case <-stopWatch:
			}
		}()

		if err := c.handshake(conn); err != nil {
			log.Printf("discord.Run: handshake error: %v", err)
			close(stopWatch)
			_ = conn.Close()
			c.emit(StateDisconnected)
			if waitErr := sleep(ctx, backoff(attempt)); waitErr != nil {
				return waitErr
			}
			attempt++
			continue
		}
		log.Printf("discord.Run: handshake ok, entering serve")

		attempt = 0
		c.emit(StateConnected)
		c.serve(ctx, conn)
		close(stopWatch)
		_ = conn.Close()
		log.Printf("discord.Run: serve returned, conn closed")
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// emit records the current connection state. State() reads it back.
func (c *Client) emit(s State) {
	c.mu.Lock()
	c.state = s
	c.mu.Unlock()
}

func (c *Client) handshake(conn io.ReadWriter) error {
	payload, err := json.Marshal(map[string]any{
		"v":         protoVersion,
		"client_id": c.clientID,
	})
	if err != nil {
		return err
	}
	log.Printf("discord.handshake: sending HANDSHAKE %d bytes", len(payload))
	if err := writeFrame(conn, opHandshake, payload); err != nil {
		return fmt.Errorf("handshake write: %w", err)
	}

	op, data, err := readFrame(conn)
	if err != nil {
		return fmt.Errorf("handshake read: %w", err)
	}
	if op == opClose {
		return fmt.Errorf("handshake closed by server: %s", string(data))
	}
	if op != opFrame {
		return fmt.Errorf("handshake unexpected opcode %d", op)
	}
	var msg struct {
		Cmd string `json:"cmd"`
		Evt string `json:"evt"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("handshake parse: %w", err)
	}
	if msg.Cmd != "DISPATCH" || msg.Evt != "READY" {
		return fmt.Errorf("handshake unexpected response cmd=%q evt=%q", msg.Cmd, msg.Evt)
	}
	log.Printf("discord.handshake: READY received")
	return nil
}

// serve sends SET_ACTIVITY on each c.sig wake-up until ctx is cancelled,
// and runs a blocking reader so a server-side close (Discord quit or
// crashed) is detected immediately instead of lingering until the next
// send. The reader is cheap: it parks in the netpoller with no CPU cost
// and no polling — the whole point of fixing the "zombie connected" state
// without a busy loop.
func (c *Client) serve(ctx context.Context, conn io.ReadWriteCloser) {
	if err := c.sendCurrent(conn); err != nil {
		c.emit(StateDisconnected)
		return
	}

	// Buffered so the reader can deliver its result and exit even after
	// serve has already returned (ctx cancelled). When serve returns,
	// Run closes conn, which unblocks the reader's readFrame.
	readErr := make(chan error, 1)
	go c.readLoop(conn, readErr)

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-readErr:
			log.Printf("discord.serve: connection closed by peer: %v", err)
			c.emit(StateDisconnected)
			return
		case <-c.sig:
			if err := c.sendCurrent(conn); err != nil {
				c.emit(StateDisconnected)
				return
			}
		}
	}
}

// readLoop drains frames from conn until it errors, delivering the cause
// on errc exactly once. It answers PINGs with PONGs and treats a CLOSE
// frame or any read error (EOF when Discord exits) as a dead connection.
// SET_ACTIVITY acknowledgements (opFrame) are read and discarded so the
// pipe's read buffer never fills.
func (c *Client) readLoop(conn io.ReadWriteCloser, errc chan<- error) {
	for {
		op, data, err := readFrame(conn)
		if err != nil {
			errc <- err
			return
		}
		switch op {
		case opPing:
			if err := c.writeFrame(conn, opPong, data); err != nil {
				errc <- err
				return
			}
		case opClose:
			errc <- fmt.Errorf("discord: connection closed by server: %s", string(data))
			return
		}
	}
}

func (c *Client) sendCurrent(w io.Writer) error {
	c.mu.Lock()
	set := c.activitySet
	var a *Activity
	if c.activity != nil {
		cp := *c.activity
		a = &cp
	}
	c.mu.Unlock()

	if !set {
		log.Printf("discord.sendCurrent: activitySet=false, skipping")
		return nil
	}

	args := map[string]any{"pid": c.pid}
	if a != nil {
		args["activity"] = a
	} else {
		args["activity"] = nil
	}
	payload := map[string]any{
		"cmd":   "SET_ACTIVITY",
		"args":  args,
		"nonce": c.nextNonce(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	log.Printf("discord.sendCurrent: sending SET_ACTIVITY %d bytes payload=%s", len(data), string(data))
	if err := c.writeFrame(w, opFrame, data); err != nil {
		log.Printf("discord.sendCurrent: writeFrame error: %v", err)
		return err
	}
	log.Printf("discord.sendCurrent: SET_ACTIVITY sent ok")
	return nil
}

func (c *Client) nextNonce() string {
	return strconv.FormatInt(c.nonce.Add(1), 10)
}

// writeFrame serializes all frame writes through writeMu so the serve
// loop and the reader goroutine never interleave bytes on the pipe.
func (c *Client) writeFrame(w io.Writer, op uint32, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return writeFrame(w, op, payload)
}

// writeFrame emits one IPC frame as a single Write so Windows synchronous
// pipes don't split the header from the payload.
func writeFrame(w io.Writer, op uint32, payload []byte) error {
	if len(payload) > maxFrameSize {
		return fmt.Errorf("discord: frame payload too large: %d", len(payload))
	}
	buf := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint32(buf[0:4], op)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(payload)))
	copy(buf[8:], payload)
	_, err := w.Write(buf)
	return err
}

func readFrame(r io.Reader) (uint32, []byte, error) {
	var hdr [8]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	op := binary.LittleEndian.Uint32(hdr[0:4])
	n := binary.LittleEndian.Uint32(hdr[4:8])
	if n > maxFrameSize {
		return 0, nil, fmt.Errorf("discord: frame too large: %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, nil, err
	}
	return op, buf, nil
}
