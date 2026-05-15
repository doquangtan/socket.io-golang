package socketio

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/doquangtan/socketio/v4/engineio"
	"github.com/doquangtan/socketio/v4/protocol"
	"github.com/gofiber/websocket/v2"
	gWebsocket "github.com/gorilla/websocket"
)

type Conn struct {
	fasthttp *websocket.Conn
	http     *gWebsocket.Conn
	polling  *protocol.Polling
}

func (c *Conn) nextWriter(messageType int) (io.WriteCloser, error) {
	if c.http != nil {
		return c.http.NextWriter(messageType)
	}
	if c.fasthttp != nil {
		return c.fasthttp.NextWriter(messageType)
	}
	if c.polling != nil {
		return c.polling.NextWriter(messageType)
	}
	return nil, errors.New("not found http or fasthttp socket")
}

func (c *Conn) setReadDeadline(t time.Time) error {
	if c.http != nil {
		return c.http.SetReadDeadline(t)
	}
	if c.fasthttp != nil {
		return c.fasthttp.SetReadDeadline(t)
	}
	return errors.New("not found http or fasthttp socket")
}

func (c *Conn) close() {
	if c.http != nil {
		c.http.Close()
	}
	if c.fasthttp != nil {
		c.fasthttp.Close()
	}
	if c.polling != nil {
		c.polling.Close()
	}
}

type broadcastOperator struct {
	sockets *connections
}

func (c *broadcastOperator) Emit(event string, agrs ...interface{}) error {
	for _, socket := range c.sockets.all() {
		socket.Emit(event, agrs...)
	}
	return nil
}

type Socket struct {
	sync.RWMutex
	Id               string
	Nps              string
	Conn             *Conn
	Handshake        engineio.Handshake
	rooms            roomNames
	listeners        listeners
	pingTime         time.Duration
	dispose          []func()
	currentNamespace func() *Namespace
	Join             func(room string)
	Leave            func(room string)
	To               func(room string) *Room
}

func (s *Socket) On(event string, fn eventCallback) {
	s.listeners.set(event, fn)
}

func (s *Socket) Emit(event string, agrs ...interface{}) error {
	c := s.Conn
	if c == nil {
		return errors.New("socket has disconnected")
	}
	agrs = append([]interface{}{event}, agrs...)
	return s.writer(protocol.EVENT, agrs)
}

func (s *Socket) Broadcast() *broadcastOperator {
	c := &broadcastOperator{
		sockets: &connections{
			conn: make(map[string]*Socket),
		},
	}
	currentNps := s.currentNamespace()
	if currentNps != nil {
		for _, socket := range currentNps.sockets.all() {
			if socket.Id != s.Id {
				c.sockets.set(socket)
			}
		}
	}
	return c
}

func (s *Socket) ack(ackEvent string, agrs ...interface{}) error {
	c := s.Conn
	if c == nil {
		return errors.New("socket has disconnected")
	}
	agrs = append([]interface{}{ackEvent}, agrs...)
	return s.writer(protocol.ACK, agrs)
}

func (s *Socket) Ping() error {
	c := s.Conn
	if c == nil {
		return errors.New("socket has disconnected")
	}
	err := s.engineWrite(engineio.PING)
	if err != nil {
		c.close()
		return err
	}
	return nil
}

func (s *Socket) Disconnect() error {
	c := s.Conn
	if c == nil {
		return errors.New("socket has disconnected")
	}
	s.writer(protocol.DISCONNECT)
	return c.setReadDeadline(time.Now())
}

func (s *Socket) Rooms() []string {
	return s.rooms.all()
}

func (s *Socket) disconnect() {
	s.Conn.close()
	s.Conn = nil
	// s.rooms = []string{}
	if len(s.dispose) > 0 {
		for _, dispose := range s.dispose {
			dispose()
		}
	}
}

func (s *Socket) engineWrite(t engineio.PacketType, arg ...interface{}) error {
	s.Lock()
	defer s.Unlock()
	w, err := s.Conn.nextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	engineio.WriteTo(w, t, arg...)
	return w.Close()
}

func (s *Socket) writer(t protocol.PacketType, arg ...interface{}) error {
	s.Lock()
	defer s.Unlock()
	w, err := s.Conn.nextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	nps := ""
	if s.Nps != "/" {
		nps = s.Nps + ","
	}
	if t == protocol.ACK {
		agrs := append([]interface{}{}, arg[0].([]interface{})[1:])
		protocol.WriteToWithAck(w, t, nps, arg[0].([]interface{})[0].(string), agrs...)
	} else {
		protocol.WriteTo(w, t, nps, arg...)
	}
	return w.Close()
}
