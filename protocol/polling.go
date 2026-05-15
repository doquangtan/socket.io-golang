package protocol

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/doquangtan/socketio/v4/engineio"
)

var (
	ErrNilPolling = errors.New("nil *Polling")
	ErrBuf        = errors.New("nil buf")
)

type closeWrapper struct {
	io.WriteCloser
	writeToBuff func(packet string)
}

// Close implements [io.WriteCloser].
func (c closeWrapper) Close() error {
	return nil
}

// Write implements [io.WriteCloser].
func (c closeWrapper) Write(p []byte) (n int, err error) {
	c.writeToBuff(string(p))
	return 0, nil
}

type Polling struct {
	mu     sync.RWMutex
	writer closeWrapper
	buf    []string
	Ready  chan struct{}
}

func (c *Polling) Push(packet string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buf = append(c.buf, packet)
	select {
	case c.Ready <- struct{}{}:
	default:
	}
}

func (c *Polling) NextWriter(messageType int) (io.WriteCloser, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c == nil {
		return nil, ErrNilPolling
	}
	if c.writer.writeToBuff == nil {
		c.writer.writeToBuff = c.Push
	}
	return c.writer, nil
}

func (c *Polling) Flush(w http.ResponseWriter) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.buf) == 0 {
		return ErrBuf
	}
	Separator := "\x1e"
	payload := strings.Join(c.buf, Separator)
	c.buf = nil
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	fmt.Fprint(w, payload)
	return nil
}

func (c *Polling) Close() error {
	c.Push(engineio.NOOP.String())
	return nil
}
