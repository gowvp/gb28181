package sip

import (
	"bytes"
	"io"
	"net"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"
)

type nopConn struct{}

func (nopConn) Read([]byte) (int, error)                  { return 0, io.EOF }
func (nopConn) Write(b []byte) (int, error)               { return len(b), nil }
func (nopConn) Close() error                              { return nil }
func (nopConn) LocalAddr() net.Addr                       { return nil }
func (nopConn) RemoteAddr() net.Addr                      { return nil }
func (nopConn) SetDeadline(time.Time) error               { return nil }
func (nopConn) SetReadDeadline(time.Time) error           { return nil }
func (nopConn) SetWriteDeadline(time.Time) error          { return nil }
func (nopConn) Network() string                           { return "udp" }
func (nopConn) ReadFrom([]byte) (int, net.Addr, error)    { return 0, nil, io.EOF }
func (nopConn) WriteTo(b []byte, _ net.Addr) (int, error) { return len(b), nil }

func ensureActiveTX() {
	if activeTX == nil {
		activeTX = &transacionts{txs: map[string]*Transaction{}, rwm: &sync.RWMutex{}}
	}
}

func countWatchGoroutines(t *testing.T) int {
	t.Helper()
	var buf bytes.Buffer
	if err := pprof.Lookup("goroutine").WriteTo(&buf, 1); err != nil {
		t.Fatal(err)
	}
	return strings.Count(buf.String(), "(*Transaction).watch")
}

func waitWatchCount(t *testing.T, want int, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if countWatchGoroutines(t) == want {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func TestWatchExitsWhenActiveClosed(t *testing.T) {
	ensureActiveTX()
	before := countWatchGoroutines(t)
	tx := NewTransaction("close-exit", nopConn{})
	if !waitWatchCount(t, before+1, time.Second) {
		t.Fatalf("want %d watch after NewTransaction, got %d", before+1, countWatchGoroutines(t))
	}

	tx.Close()

	if waitWatchCount(t, before, time.Second) {
		return
	}
	t.Fatalf("watch still running after Close (left %d)", countWatchGoroutines(t)-before)
}

func TestCloseIdempotent(t *testing.T) {
	ensureActiveTX()
	tx := NewTransaction("close-once", nopConn{})
	tx.Close()
	tx.Close()
}
