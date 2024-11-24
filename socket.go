package socketio

import (
	"errors"
	"time"

	"github.com/doquangtan/gofiber-socket.io/engineio"
	"github.com/doquangtan/gofiber-socket.io/socket_protocol"
	"github.com/gofiber/websocket/v2"
)

type Socket struct {
	Id        string
	Nps       string
	Conn      *websocket.Conn
	rooms     []string
	listeners listeners
	pingTime  time.Duration
	dispose   []func()
}

func (s *Socket) On(event string, fn eventCallback) {
	s.listeners.set(event, fn)
}

func (s *Socket) Emit(event string, agrs ...interface{}) error {
	c := s.Conn
	if c == nil || c.Conn == nil {
		return errors.New("socket has disconnected")
	}
	agrs = append([]interface{}{event}, agrs...)
	return s.writer(socket_protocol.EVENT, agrs)
}

func (s *Socket) Ping() error {
	c := s.Conn
	if c == nil || c.Conn == nil {
		return errors.New("socket has disconnected")
	}
	w, err := c.Conn.NextWriter(websocket.TextMessage)
	if err != nil {
		c.Close()
		return err
	}
	engineio.WriteByte(w, engineio.PING, []byte{})
	return w.Close()
}

func (s *Socket) Disconnect() error {
	c := s.Conn
	if c == nil || c.Conn == nil {
		return errors.New("socket has disconnected")
	}
	s.writer(socket_protocol.DISCONNECT)
	return s.Conn.SetReadDeadline(time.Now())
}

func (s *Socket) disconnect() {
	s.Conn.Close()
	s.Conn = nil
	s.rooms = []string{}
	if len(s.dispose) > 0 {
		for _, dispose := range s.dispose {
			dispose()
		}
	}
}

func (s *Socket) engineWrite(t engineio.PacketType, arg ...interface{}) error {
	w, err := s.Conn.Conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	engineio.WriteTo(w, t, arg...)
	return w.Close()
}

func (s *Socket) writer(t socket_protocol.PacketType, arg ...interface{}) error {
	w, err := s.Conn.Conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	nps := ""
	if s.Nps != "/" {
		nps = s.Nps + ","
	}
	socket_protocol.WriteTo(w, t, nps, arg...)
	return w.Close()
}