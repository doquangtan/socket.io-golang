package socketio

import (
	"errors"
	"sync"
)

var (
	ErrorInvalidConnection = errors.New("invalid connection")
	ErrorUUIDDuplication   = errors.New("UUID already exists")
)

type connections struct {
	sync.RWMutex
	conn map[string]*Socket
}

func (l *connections) set(socket *Socket) error {
	l.Lock()
	if l.conn[socket.Id] != nil {
		return ErrorUUIDDuplication
	}
	l.conn[socket.Id] = socket
	l.Unlock()
	return nil
}

func (l *connections) get(id string) (*Socket, error) {
	l.RLock()
	defer l.RUnlock()
	ret, ok := l.conn[id]
	if !ok {
		return nil, ErrorInvalidConnection
	}
	return ret, nil
}

func (l *connections) all() []*Socket {
	l.RLock()
	defer l.RUnlock()
	ret := make([]*Socket, 0)
	for _, socket := range l.conn {
		ret = append(ret, socket)
	}
	return ret
}

func (l *connections) delete(key string) {
	l.Lock()
	delete(l.conn, key)
	l.Unlock()
}
