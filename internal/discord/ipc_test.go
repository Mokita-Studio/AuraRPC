package discord

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte(`{"hello":"world"}`)
	if err := writeFrame(&buf, opFrame, payload); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 8+len(payload) {
		t.Fatalf("frame length = %d, want %d", buf.Len(), 8+len(payload))
	}

	op, data, err := readFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if op != opFrame {
		t.Errorf("op = %d, want %d", op, opFrame)
	}
	if !bytes.Equal(data, payload) {
		t.Errorf("data = %s, want %s", data, payload)
	}
}

func TestReadFrameRejectsOversize(t *testing.T) {
	var hdr [8]byte
	binary.LittleEndian.PutUint32(hdr[0:4], opFrame)
	binary.LittleEndian.PutUint32(hdr[4:8], maxFrameSize+1)

	if _, _, err := readFrame(bytes.NewReader(hdr[:])); err == nil {
		t.Fatal("expected error for oversize frame")
	}
}

func TestReadFrameTruncated(t *testing.T) {
	if _, _, err := readFrame(bytes.NewReader([]byte{1, 2, 3})); err == nil {
		t.Fatal("expected error for truncated header")
	}
}

func TestSendCurrentSkipsWhenUnset(t *testing.T) {
	var buf bytes.Buffer
	c := New("123")
	if err := c.sendCurrent(&buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no write, got %d bytes", buf.Len())
	}
}

func TestSendCurrentActivity(t *testing.T) {
	var buf bytes.Buffer
	c := New("999")
	c.SetActivity(Activity{Details: "Hello", State: "World"})

	if err := c.sendCurrent(&buf); err != nil {
		t.Fatal(err)
	}

	op, data, err := readFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if op != opFrame {
		t.Errorf("op = %d, want %d", op, opFrame)
	}

	var msg struct {
		Cmd  string `json:"cmd"`
		Args struct {
			PID      int `json:"pid"`
			Activity struct {
				Details string `json:"details"`
				State   string `json:"state"`
			} `json:"activity"`
		} `json:"args"`
		Nonce string `json:"nonce"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Cmd != "SET_ACTIVITY" {
		t.Errorf("cmd = %s", msg.Cmd)
	}
	if msg.Args.Activity.Details != "Hello" || msg.Args.Activity.State != "World" {
		t.Errorf("activity payload: %+v", msg.Args.Activity)
	}
	if msg.Nonce == "" {
		t.Error("nonce empty")
	}
}

func TestHandshakeSuccess(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	client := New("client-id-42")

	done := make(chan error, 1)
	go func() { done <- client.handshake(c1) }()

	op, data, err := readFrame(c2)
	if err != nil {
		t.Fatal(err)
	}
	if op != opHandshake {
		t.Fatalf("expected opHandshake, got %d", op)
	}
	var hs map[string]any
	if err := json.Unmarshal(data, &hs); err != nil {
		t.Fatal(err)
	}
	if hs["client_id"] != "client-id-42" {
		t.Errorf("client_id = %v", hs["client_id"])
	}
	if v, _ := hs["v"].(float64); int(v) != protoVersion {
		t.Errorf("v = %v", hs["v"])
	}

	ready, _ := json.Marshal(map[string]string{"cmd": "DISPATCH", "evt": "READY"})
	if err := writeFrame(c2, opFrame, ready); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handshake: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handshake timed out")
	}
}

func TestHandshakeRejectsBadResponse(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	client := New("x")
	done := make(chan error, 1)
	go func() { done <- client.handshake(c1) }()

	if _, _, err := readFrame(c2); err != nil {
		t.Fatal(err)
	}
	bad, _ := json.Marshal(map[string]string{"cmd": "DISPATCH", "evt": "NOPE"})
	if err := writeFrame(c2, opFrame, bad); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected handshake error")
		}
	case <-time.After(time.Second):
		t.Fatal("handshake timed out")
	}
}

func TestServeResendsOnSignal(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	client := New("42")
	client.SetActivity(Activity{Details: "first"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		client.serve(ctx, c1)
		close(done)
	}()

	if _, data, err := readFrame(c2); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(string(data), `"first"`) {
		t.Errorf("expected first activity, got %s", data)
	}

	client.SetActivity(Activity{Details: "second"})

	if _, data, err := readFrame(c2); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(string(data), `"second"`) {
		t.Errorf("expected second activity, got %s", data)
	}

	cancel()
	c1.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("serve did not return after cancel")
	}
}

func TestServeReturnsOnCancel(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	client := New("x")
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		client.serve(ctx, c1)
		close(done)
	}()

	cancel()
	c1.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("serve did not return after cancel")
	}
	_ = c2
}

// TestServeDetectsPeerClose is the regression test for the "zombie
// connected" state: when Discord exits, the background reader must notice
// the closed pipe immediately, without the client having to send anything.
func TestServeDetectsPeerClose(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()

	client := New("42")
	client.SetActivity(Activity{Details: "live"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		client.serve(ctx, c1)
		close(done)
	}()

	// Drain the initial SET_ACTIVITY so serve is parked in its select.
	if _, _, err := readFrame(c2); err != nil {
		t.Fatal(err)
	}

	// Simulate Discord exiting. No further send is issued by the client,
	// so only the reader can detect this.
	c2.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("serve did not return after peer close — zombie state regressed")
	}
	if client.State() != StateDisconnected {
		t.Errorf("state = %v, want disconnected", client.State())
	}
}

// TestServeAnswersPing checks the reader echoes a PING back as a PONG
// through the serialized write path.
func TestServeAnswersPing(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	client := New("42")
	client.SetActivity(Activity{Details: "live"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.serve(ctx, c1)

	if _, _, err := readFrame(c2); err != nil {
		t.Fatal(err)
	}
	if err := writeFrame(c2, opPing, []byte("ping-data")); err != nil {
		t.Fatal(err)
	}

	// A pending SetActivity signal can emit one extra SET_ACTIVITY frame
	// before the PONG; drain opFrame frames until the PONG arrives.
	deadline := time.After(2 * time.Second)
	for {
		op, data, err := readFrame(c2)
		if err != nil {
			t.Fatal(err)
		}
		if op == opFrame {
			select {
			case <-deadline:
				t.Fatal("never received PONG")
			default:
			}
			continue
		}
		if op != opPong {
			t.Fatalf("op = %d, want opPong %d", op, opPong)
		}
		if string(data) != "ping-data" {
			t.Errorf("pong data = %q, want %q", data, "ping-data")
		}
		return
	}
}

func TestWriteFrameRejectsOversize(t *testing.T) {
	var buf bytes.Buffer
	huge := make([]byte, maxFrameSize+1)
	if err := writeFrame(&buf, opFrame, huge); err == nil {
		t.Fatal("expected error for oversize payload")
	}
}

func TestEmitTracksState(t *testing.T) {
	c := New("x")
	if c.State() != StateDisconnected {
		t.Errorf("initial state = %v", c.State())
	}
	c.emit(StateConnecting)
	if c.State() != StateConnecting {
		t.Errorf("state after emit = %v", c.State())
	}
	c.emit(StateConnected)
	if c.State() != StateConnected {
		t.Errorf("state after emit = %v", c.State())
	}
}

func TestRunHappyPath(t *testing.T) {
	c1, c2 := net.Pipe()

	oldDial := dial
	dial = func() (io.ReadWriteCloser, error) { return c1, nil }
	defer func() { dial = oldDial }()

	client := New("rid")
	client.SetActivity(Activity{Details: "running"})

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- client.Run(ctx) }()

	if _, _, err := readFrame(c2); err != nil {
		t.Fatal(err)
	}
	ready, _ := json.Marshal(map[string]string{"cmd": "DISPATCH", "evt": "READY"})
	if err := writeFrame(c2, opFrame, ready); err != nil {
		t.Fatal(err)
	}

	if _, data, err := readFrame(c2); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(string(data), `"running"`) {
		t.Errorf("expected activity, got %s", data)
	}

	cancel()
	c1.Close()
	c2.Close()

	select {
	case err := <-runDone:
		if err != context.Canceled {
			t.Errorf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

// TestRunSetActivityAfterHandshakeStart mirrors the real Controller flow:
// SetActivity is called AFTER the run goroutine is already inside the
// handshake. The first sendCurrent (on entering serve) must see
// activitySet == true and emit SET_ACTIVITY.
func TestRunSetActivityAfterHandshakeStart(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()

	oldDial := dial
	dial = func() (io.ReadWriteCloser, error) { return c1, nil }
	defer func() { dial = oldDial }()

	client := New("rid")

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- client.Run(ctx) }()

	// Wait for the goroutine to write HANDSHAKE; it is now blocked in
	// handshake.read waiting for READY.
	if _, _, err := readFrame(c2); err != nil {
		t.Fatal(err)
	}

	// Mirror ctrl.Apply: SetActivity called from the main thread while
	// the run goroutine is blocked.
	client.SetActivity(Activity{Details: "after-handshake-start"})

	// Unblock the handshake with READY.
	ready, _ := json.Marshal(map[string]string{"cmd": "DISPATCH", "evt": "READY"})
	if err := writeFrame(c2, opFrame, ready); err != nil {
		t.Fatal(err)
	}

	// Then wait for the SET_ACTIVITY frame.
	deadline := time.After(2 * time.Second)
	for {
		op, data, err := readFrame(c2)
		if err != nil {
			t.Fatalf("readFrame: %v", err)
		}
		if op == opFrame && strings.Contains(string(data), "after-handshake-start") {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("never received SET_ACTIVITY with expected payload")
		default:
		}
	}

	cancel()
	c1.Close()
	select {
	case <-runDone:
	case <-time.After(time.Second):
		t.Fatal("Run did not return")
	}
}

// TestRunClosesConnOnCancel reproduces the freeze scenario: ctx is
// cancelled while a readFrame is in flight; Run must release the
// connection immediately without waiting for the peer.
func TestRunClosesConnOnCancel(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()

	oldDial := dial
	dial = func() (io.ReadWriteCloser, error) { return c1, nil }
	defer func() { dial = oldDial }()

	client := New("rid")
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- client.Run(ctx) }()

	// Complete the handshake so the goroutine enters serve and parks in
	// a select; without the cancel watcher the read would never wake.
	if _, _, err := readFrame(c2); err != nil {
		t.Fatal(err)
	}
	ready, _ := json.Marshal(map[string]string{"cmd": "DISPATCH", "evt": "READY"})
	if err := writeFrame(c2, opFrame, ready); err != nil {
		t.Fatal(err)
	}

	// Wait until the client transitions to connected.
	deadline := time.After(time.Second)
	for client.State() != StateConnected {
		select {
		case <-deadline:
			t.Fatal("never reached Connected")
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	select {
	case err := <-runDone:
		if err != context.Canceled {
			t.Errorf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not return promptly after cancel — watcher failed to close conn")
	}
}

// TestRunUnblocksHandshakeOnCancel is the regression test for the pipe/
// goroutine leak on rapid restart: a client cancelled while blocked in the
// handshake read (no READY ever arrives) must return promptly instead of
// holding its pipe until Discord answers.
func TestRunUnblocksHandshakeOnCancel(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()

	oldDial := dial
	dial = func() (io.ReadWriteCloser, error) { return c1, nil }
	defer func() { dial = oldDial }()

	client := New("rid")
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- client.Run(ctx) }()

	// Wait for the HANDSHAKE write; Run is now blocked reading READY,
	// which we deliberately never send.
	if _, _, err := readFrame(c2); err != nil {
		t.Fatal(err)
	}

	cancel()
	select {
	case err := <-runDone:
		if err != context.Canceled {
			t.Errorf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancel during handshake — pipe/goroutine leak")
	}
}

func TestStateString(t *testing.T) {
	cases := map[State]string{
		StateDisconnected: "disconnected",
		StateConnecting:   "connecting",
		StateConnected:    "connected",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("%d.String() = %s, want %s", s, got, want)
		}
	}
}
