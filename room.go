package socketio

import (
	"sync"
)

type Room struct {
	Name           string
	To             func(room string) *Room
	sockets        connections
	connectSockets []*Socket
}

func newRoom(name string) *Room {
	return &Room{
		Name: name,
		sockets: connections{
			conn: make(map[string]*Socket),
		},
	}
}

func (room *Room) Emit(event string, agrs ...interface{}) error {
	if len(room.connectSockets) > 0 {
		for _, socket := range room.connectSockets {
			socket.Emit(event, agrs...)
		}
	} else {
		for _, socket := range room.sockets.all() {
			socket.Emit(event, agrs...)
		}
	}
	return nil
}

func (room *Room) Sockets() []*Socket {
	if len(room.connectSockets) > 0 {
		return room.connectSockets
	}
	return room.sockets.all()
}

type roomNames struct {
	sync.RWMutex
	list []string
}

func (l *roomNames) set(name string) {
	l.Lock()
	defer l.Unlock()
	l.list = append(l.list, name)
}

func (l *roomNames) delete(name string) int {
	l.Lock()
	defer l.Unlock()
	for index, n := range l.list {
		if n == name {
			l.list = append(l.list[:index], l.list[index+1:]...)
			return index
		}
	}
	return -1
}

func (l *roomNames) all() []string {
	l.RLock()
	defer l.RUnlock()
	return append([]string{}, l.list...)
}

type rooms struct {
	sync.RWMutex
	list map[string]*Room
}

func (n *rooms) create(name string) *Room {
	n.Lock()
	defer n.Unlock()
	ret, ok := n.list[name]
	if !ok {
		ret = newRoom(name)
		ret.To = func(room string) *Room {
			return n.next(room, ret)
		}
		n.list[name] = ret
	}
	return ret
}

func (n *rooms) next(name string, preRoom ...*Room) *Room {
	persistRoom := n.create(name)
	persistSockets := persistRoom.sockets.all()

	if len(preRoom) == 0 {
		target := newRoom(name)
		target.connectSockets = persistSockets
		target.To = func(room string) *Room {
			return n.next(room, target)
		}
		return target
	}

	prev := preRoom[0]
	chained := newRoom(prev.Name + "_" + persistRoom.Name)

	seen := make(map[string]bool)
	var allSockets []*Socket

	prevSockets := prev.connectSockets
	if len(prevSockets) == 0 {
		prevSockets = prev.sockets.all()
	}
	for _, s := range prevSockets {
		if s != nil && !seen[s.Id] {
			seen[s.Id] = true
			allSockets = append(allSockets, s)
		}
	}
	for _, s := range persistSockets {
		if s != nil && !seen[s.Id] {
			seen[s.Id] = true
			allSockets = append(allSockets, s)
		}
	}

	chained.connectSockets = allSockets
	chained.To = func(room string) *Room {
		return n.next(room, chained)
	}
	return chained
}

// func (n *rooms) get(name string) *Room {
// 	n.RLock()
// 	defer n.RUnlock()
// 	ret, ok := n.list[name]
// 	if !ok {
// 		return nil
// 	}
// 	return ret
// }
