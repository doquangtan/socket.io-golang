package client

import "time"

// type payload struct {
// 	socket *Socket
// 	data   interface{}
// 	ackId  string
// }

type Io struct {
	pingInterval time.Duration
	pingTimeout  time.Duration
	maxPayload   int
	// namespaces       namespaces
	// sockets          connections
	// readChan         chan payload
	// onAuthentication func(params map[string]string) bool
	// onConnection     connectionEvent
	// close            chan interface{}
}

func New() *Io {
	pingInterval := time.Duration(25000 * time.Millisecond)
	pingTimeout := time.Duration(25000 * time.Millisecond)
	maxPayload := 1000000
	io := &Io{
		// readChan: make(chan payload),
		// close:    make(chan interface{}),
		// onConnection: connectionEvent{
		// 	list: make(map[string][]connectionEventCallback),
		// },
		// namespaces: namespaces{
		// 	list: make(map[string]*Namespace),
		// },
		// sockets: connections{
		// 	conn: make(map[string]*Socket),
		// },
		pingInterval: pingInterval,
		pingTimeout:  pingTimeout,
		maxPayload:   maxPayload,
	}
	// ctx, cancelFunc := context.WithCancel(context.Background())
	// go io.read(ctx)
	// go io.ping(ctx)
	// go func() {
	// 	<-io.close
	// 	cancelFunc()
	// }()
	return io
}
