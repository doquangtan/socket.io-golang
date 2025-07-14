package socketio

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
	gWebsocket "github.com/gorilla/websocket"
	"github.com/lib4u/socket.io-golang/v4/engineio"
	"github.com/lib4u/socket.io-golang/v4/socket_protocol"
)

type Conn struct {
	fasthttp *websocket.Conn
	http     *gWebsocket.Conn
}

func (c *Conn) NextWriter(messageType int) (io.WriteCloser, error) {
	if c.http != nil {
		return c.http.NextWriter(messageType)
	}
	if c.fasthttp != nil {
		return c.fasthttp.NextWriter(messageType)
	}
	return nil, errors.New("not found http or fasthttp socket")
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	if c.http != nil {
		return c.http.SetReadDeadline(t)
	}
	if c.fasthttp != nil {
		return c.fasthttp.SetReadDeadline(t)
	}
	return errors.New("not found http or fasthttp socket")
}

func (c *Conn) Close() error {
	if c.http != nil {
		return c.http.Close()
	}
	if c.fasthttp != nil {
		return c.fasthttp.Close()
	}
	return errors.New("not found http or fasthttp socket")
}

type Socket struct {
	Id        string
	Nps       string
	Conn      *Conn
	rooms     roomNames
	listeners listeners
	pingTime  time.Duration
	dispose   []func()
	Join      func(room string)
	Leave     func(room string)
	To        func(room string) *Room
	mu        sync.Mutex
}

func (s *Socket) On(event string, fn eventCallback) {
	s.listeners.set(event, fn)
}

func (s *Socket) Emit(event string, args ...interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Conn == nil {
		return errors.New("socket has disconnected")
	}

	payload := make([]interface{}, 1+len(args))
	payload[0] = event
	copy(payload[1:], args)
	return s.writerUnsafe(socket_protocol.EVENT, payload)
}

func (s *Socket) ack(event string, args ...interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Conn == nil {
		return errors.New("socket has disconnected")
	}

	payload := make([]interface{}, 1+len(args))
	payload[0] = event
	copy(payload[1:], args)
	return s.writerUnsafe(socket_protocol.ACK, payload)
}

func (s *Socket) Ping() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.Conn
	if c == nil {
		return errors.New("socket has disconnected")
	}
	w, err := c.NextWriter(websocket.TextMessage)
	if err != nil {
		c.Close()
		return err
	}
	engineio.WriteByte(w, engineio.PING, []byte{})
	return w.Close()
}

func (s *Socket) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Conn == nil {
		return errors.New("socket has disconnected")
	}

	if err := s.writerUnsafe(socket_protocol.DISCONNECT); err != nil {
		return err
	}
	return s.Conn.SetReadDeadline(time.Now())
}

func (s *Socket) Rooms() []string {
	return s.rooms.all()
}

func (s *Socket) disconnect() {
	s.Conn.Close()
	s.Conn = nil
	// s.rooms = []string{}
	if len(s.dispose) > 0 {
		for _, dispose := range s.dispose {
			dispose()
		}
	}
}

func (s *Socket) engineWrite(t engineio.PacketType, arg ...interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.Conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	engineio.WriteTo(w, t, arg...)
	return w.Close()
}

func (s *Socket) writer(t socket_protocol.PacketType, arg ...interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writerUnsafe(t, arg...)
}

func (s *Socket) writerUnsafe(t socket_protocol.PacketType, arg ...interface{}) error {
	w, err := s.Conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	nps := ""
	if s.Nps != "/" {
		nps = s.Nps + ","
	}
	if t == socket_protocol.ACK {
		agrs := append([]interface{}{}, arg[0].([]interface{})[1:])
		socket_protocol.WriteToWithAck(w, t, nps, arg[0].([]interface{})[0].(string), agrs...)
	} else {
		socket_protocol.WriteTo(w, t, nps, arg...)
	}
	return w.Close()
}
