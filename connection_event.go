package socketio

import (
	"sync"
)

type connectionEventCallback func(payload *Socket, paramsRow map[string]string)

type connectionEvent struct {
	sync.RWMutex
	list map[string][]connectionEventCallback
}

func (l *connectionEvent) set(event string, callback connectionEventCallback) {
	l.Lock()
	l.list[event] = append(l.list[event], callback)
	l.Unlock()
}

func (l *connectionEvent) get(event string) []connectionEventCallback {
	l.RLock()
	defer l.RUnlock()
	if _, ok := l.list[event]; !ok {
		return make([]connectionEventCallback, 0)
	}
	ret := make([]connectionEventCallback, 0)
	ret = append(ret, l.list[event]...)
	return ret
}
