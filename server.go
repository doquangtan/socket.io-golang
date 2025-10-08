package socketio

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/doquangtan/socketio/v4/client"
	"github.com/doquangtan/socketio/v4/engineio"
	"github.com/doquangtan/socketio/v4/socket_protocol"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"

	gWebsocket "github.com/gorilla/websocket"
)

// Create socket-client
func Connect() *client.Io {
	return client.New()
}

//go:embed client-dist/*
var staticFS embed.FS

type payload struct {
	socket *Socket
	data   interface{}
	ackId  string
}

type Io struct {
	pingInterval     time.Duration
	pingTimeout      time.Duration
	maxPayload       int
	namespaces       namespaces
	sockets          connections
	readChan         chan payload
	onAuthentication func(params map[string]string) bool
	onConnection     connectionEvent
	close            chan interface{}
}

func New() *Io {
	pingInterval := time.Duration(25000 * time.Millisecond)
	pingTimeout := time.Duration(25000 * time.Millisecond)
	maxPayload := 1000000
	io := &Io{
		readChan: make(chan payload),
		close:    make(chan interface{}),
		onConnection: connectionEvent{
			list: make(map[string][]connectionEventCallback),
		},
		namespaces: namespaces{
			list: make(map[string]*Namespace),
		},
		sockets: connections{
			conn: make(map[string]*Socket),
		},
		pingInterval: pingInterval,
		pingTimeout:  pingTimeout,
		maxPayload:   maxPayload,
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	go io.read(ctx)
	go io.ping(ctx)
	go func() {
		<-io.close
		cancelFunc()
	}()
	return io
}

var upgrader = gWebsocket.Upgrader{}

func (s *Io) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := r.Header
	if slices.ContainsFunc(header["Connection"], func(s string) bool { return strings.Contains(s,"Upgrade") }) && header.Get("Upgrade") == "websocket" {
		upgrader.CheckOrigin = func(r *http.Request) bool { return true }
		c, err := upgrader.Upgrade(w, r, nil)

		if err != nil {
			log.Print("Upgrade:", err)
			return
		}
		defer c.Close()

		if r.URL.Query()["sid"] != nil {
			return
		}

		socket := Socket{
			Id:  s.randomUUID(),
			Nps: "/",
			Conn: &Conn{
				http: c,
			},
			listeners: listeners{
				list: make(map[string][]eventCallback),
			},
			pingTime: s.pingInterval,
		}
		defer socket.disconnect()
		socket.dispose = append(socket.dispose, func() {
			s.sockets.delete(socket.Id)
		})
		s.sockets.set(&socket)

		socket.engineWrite(engineio.OPEN, engineio.ConnParameters{
			SID:          socket.Id,
			PingInterval: s.pingInterval,
			PingTimeout:  s.pingTimeout,
			MaxPayload:   s.maxPayload,
			Upgrades:     []string{"websocket"},
		}.ToJson())

		for {
			messageType, message, err := c.ReadMessage()
			if err != nil {
				break
			}

			if messageType == websocket.TextMessage {
				err := s.handlerMessage(&socket, string(message))
				if err != nil {
					return
				}
			}
		}
	} else if strings.HasPrefix(r.URL.Path, "/socket.io/") {
		clientDistFs, _ := fs.Sub(staticFS, "client-dist")
		fs := http.StripPrefix("/socket.io/", http.FileServer(http.FS(clientDistFs)))
		fs.ServeHTTP(w, r)
	} else {
		http.NotFound(w, r)
	}
}

func (s *Io) HttpHandler() http.Handler {
	return s
}

func (s *Io) FiberRoute(router fiber.Router) {
	clientDistFs, _ := fs.Sub(staticFS, "client-dist")
	router.Use("/", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		} else if strings.HasPrefix(c.Path(), "/socket.io/") {
			fileName := strings.Replace(c.Path(), "/socket.io/", "", 1)
			return filesystem.SendFile(c, http.FS(clientDistFs), fileName)
		}
		return fiber.ErrUpgradeRequired
	})
	router.Get("/", s.new())
}

func (s *Io) FiberMiddleware(c *fiber.Ctx) error {
	if c.Locals("io") == nil {
		c.Locals("io", s)
	}
	return c.Next()
}

func (s *Io) Close() {
	s.close <- true
}

func (s *Io) Of(name string) *Namespace {
	return s.namespaces.create(name)
}

func (s *Io) To(name string) *Room {
	return s.Of("/").To(name)
}

func (s *Io) Sockets() []*Socket {
	return s.Of("/").Sockets()
}

func (s *Io) OnConnection(fn connectionEventCallback) {
	s.Of("/").onConnection.set("connection", fn)
}

func (s *Io) OnAuthentication(fn func(params map[string]string) bool) {
	s.onAuthentication = fn
}

func (s *Io) Emit(event string, agrs ...interface{}) error {
	return s.Of("/").Emit(event, agrs...)
}

func (s *Io) read(ctx context.Context) {
	for {
		select {
		case payLoad := <-s.readChan:
			if payLoad.socket.Conn == nil {
				continue
			}
			dataJson := []interface{}{}
			json.Unmarshal([]byte(payLoad.data.(string)), &dataJson)
			if len(dataJson) > 0 {
				if reflect.TypeOf(dataJson[0]).String() == "string" {
					event := dataJson[0].(string)
					for _, callback := range payLoad.socket.listeners.get(event) {
						data := append([]interface{}{}, dataJson[1:]...)

						ackCallback := AckCallback(func(data ...interface{}) {
							payLoad.socket.ack(payLoad.ackId, data...)
						})

						if payLoad.ackId == "" {
							callback(&EventPayload{
								SID:    payLoad.socket.Id,
								Name:   event,
								Socket: payLoad.socket,
								Error:  nil,
								Data:   data,
								Ack:    nil,
							})
						} else {
							callback(&EventPayload{
								SID:    payLoad.socket.Id,
								Name:   event,
								Socket: payLoad.socket,
								Error:  nil,
								Data:   data,
								Ack:    ackCallback,
							})
						}
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Io) ping(ctx context.Context) {
	timeoutTicker := time.NewTicker(time.Duration(1 * time.Second))
	defer timeoutTicker.Stop()
	for {
		select {
		case <-timeoutTicker.C:
			for _, socket := range s.sockets.all() {
				if socket != nil && socket.pingTime > 0 {
					socket.pingTime = time.Duration(socket.pingTime - time.Duration(1*time.Second))
					if socket.pingTime <= 0 {
						err := socket.Ping()
						if err != nil {
							s.sockets.delete(socket.Id)
						} else {
							socket.pingTime = s.pingInterval
						}
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Io) randomUUID() string {
	return uuid.New().String()
}

func (s *Io) new() func(ctx *fiber.Ctx) error {
	return websocket.New(func(c *websocket.Conn) {
		if c.Query("sid") != "" {
			return
		}

		socket := Socket{
			Id:  s.randomUUID(),
			Nps: "/",
			Conn: &Conn{
				fasthttp: c,
			},
			listeners: listeners{
				list: make(map[string][]eventCallback),
			},
			pingTime: s.pingInterval,
		}
		defer socket.disconnect()
		socket.dispose = append(socket.dispose, func() {
			s.sockets.delete(socket.Id)
		})
		s.sockets.set(&socket)

		socket.engineWrite(engineio.OPEN, engineio.ConnParameters{
			SID:          socket.Id,
			PingInterval: s.pingInterval,
			PingTimeout:  s.pingTimeout,
			MaxPayload:   s.maxPayload,
			Upgrades:     []string{"websocket"},
		}.ToJson())

		for {
			messageType, message, err := c.ReadMessage()
			if err != nil {
				// if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// }
				return
			}

			if messageType == websocket.TextMessage {
				err := s.handlerMessage(&socket, string(message))
				if err != nil {
					return
				}
			}
		}
	})
}

func (s *Io) handlerMessage(socket *Socket, message string) error {
	enginePacketType := string(message[0:1])
	switch enginePacketType {
	case engineio.MESSAGE.String():
		mess := string(message)
		packetType := string(message[1:2])
		rawpayload := string(message[2:])

		endNamespace := -1
		startPayload := -1
		ackId := ""

		special1 := strings.Index(mess, "{")
		special2 := strings.Index(mess, "[")
		special3 := -1
		nextMess := message

		for {
			nextSpecial3 := strings.Index(string(nextMess), ",")
			if nextSpecial3 == -1 || (special1 != -1 && nextSpecial3 > special1) || (special2 != -1 && nextSpecial3 > special2) {
				break
			}
			nextMess = nextMess[nextSpecial3+1:]
			special3 = nextSpecial3
		}

		if special3 != -1 {
			endNamespace = special3
		}

		startPayload = endNamespace
		if special2 != -1 {
			startPayload = special2 - 1
		} else if special1 != -1 {
			startPayload = special1 - 1
		}

		if special3 != -1 && special2 != -1 && (special2-1 != special3) {
			ackId = string(message[special3+1 : special2])
		} else if special2 != -1 && special2 != 2 {
			ackId = string(message[2:special2])
		}

		namespace := "/"
		if endNamespace != -1 {
			namespace = string(message[2:endNamespace])
		}

		if startPayload != -1 {
			rawpayload = string(message[startPayload+1:])
		}

		switch packetType {
		case socket_protocol.DISCONNECT.String():
			socket_nps, err := s.Of(namespace).sockets.get(socket.Id)
			if err != nil {
				return err
			}
			for _, callback := range socket_nps.listeners.get("disconnecting") {
				callback(&EventPayload{
					SID:    socket.Id,
					Name:   "disconnecting",
					Socket: socket_nps,
					Error:  nil,
					Data:   []interface{}{},
				})
			}
		case socket_protocol.CONNECT.String():
			socket_nps := socket
			if namespace != "/" {
				socketWithNamespace := Socket{
					Id:   socket.Id,
					Nps:  namespace,
					Conn: socket.Conn,
					listeners: listeners{
						list: make(map[string][]eventCallback),
					},
					pingTime: s.pingInterval,
				}
				socket_nps = &socketWithNamespace

				if nps := s.namespaces.get(namespace); nps == nil {
					socket_nps.writer(socket_protocol.CONNECT_ERROR, map[string]interface{}{
						"message": "Invalid namespace",
					})
					// continue
					return nil
				}
			}

			if s.onAuthentication != nil {
				dataJson := map[string]string{}
				json.Unmarshal([]byte(rawpayload), &dataJson)
				if !s.onAuthentication(dataJson) {
					socket_nps.writer(socket_protocol.CONNECT_ERROR, map[string]interface{}{
						"message": "Not authenticated",
					})
					// continue
					return nil
				}
			}

			socket.dispose = append(socket.dispose, func() {
				s.Of(namespace).socketLeaveAllRooms(socket_nps)
				s.Of(namespace).sockets.delete(socket_nps.Id)
				for _, callback := range socket_nps.listeners.get("disconnect") {
					callback(&EventPayload{
						SID:    socket_nps.Id,
						Name:   "disconnect",
						Socket: socket_nps,
						Error:  nil,
						Data:   []interface{}{},
					})
				}
			})
			s.Of(namespace).sockets.set(socket_nps)
			socket_nps.Join = func(room string) {
				s.Of(namespace).socketJoinRoom(room, socket_nps)
			}
			socket_nps.Leave = func(room string) {
				s.Of(namespace).socketLeaveRoom(room, socket_nps)
			}
			socket_nps.To = func(room string) *Room {
				return s.Of(namespace).To(room)
			}

			socket_nps.writer(socket_protocol.CONNECT, engineio.ConnParameters{
				SID: socket.Id,
			}.ToJson())

			for _, callback := range s.Of(namespace).onConnection.get("connection") {
				callback(socket_nps)
			}
		case socket_protocol.EVENT.String():
			socket_nps, err := s.Of(namespace).sockets.get(socket.Id)
			if err != nil {
				return err
			}
			if socket.Conn != nil {
				s.readChan <- payload{
					socket: socket_nps,
					data:   rawpayload,
					ackId:  ackId,
				}
			}
			// case socket_protocol.BINARY_EVENT.String():
			// 	socket_nps, err := s.Of(namespace).sockets.get(socket.Id)
			// 	if err != nil {
			// 		return err
			// 	}
			// 	log.Println("Debug: ", rawpayload)
			// 	if socket.Conn != nil {
			// 		s.readChan <- payload{
			// 			socket: socket_nps,
			// 			data:   rawpayload,
			// 			ackId:  ackId,
			// 		}
			// 	}
			// case socket_protocol.BINARY_ACK.String():
		}
	case engineio.PONG.String():
		// println("Client pong")
	}
	return nil
}
