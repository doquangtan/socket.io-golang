package socketio

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/doquangtan/socketio/v4/client"
	"github.com/doquangtan/socketio/v4/engineio"
	"github.com/doquangtan/socketio/v4/protocol"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
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

type UseError struct {
	Message string
	Data    map[string]interface{}
}

func (e UseError) Error() string {
	return e.Message
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
	use              func(socket *Socket, next func() *UseError) *UseError
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
	query := r.URL.Query()
	transport := query.Get("transport")
	sid := query.Get("sid")
	if transport == "websocket" &&
		slices.Contains(header["Connection"], "Upgrade") &&
		header.Get("Upgrade") == "websocket" {
		upgrader.CheckOrigin = func(r *http.Request) bool { return true }
		c, err := upgrader.Upgrade(w, r, nil)

		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer c.Close()

		var socket *Socket
		if sid != "" {
			socket, err = s.sockets.get(sid)
			if err != nil {
				http.Error(w, "unknown session", http.StatusBadRequest)
				return
			}
			socket.Conn.http = c
			defer socket.disconnect()
		} else {
			socket = &Socket{
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
			s.sockets.set(socket)

			socket.engineWrite(engineio.OPEN, engineio.ConnParameters{
				SID:          socket.Id,
				PingInterval: s.pingInterval,
				PingTimeout:  s.pingTimeout,
				MaxPayload:   s.maxPayload,
				Upgrades:     []string{},
			}.ToJson())
		}

		for {
			messageType, message, err := c.ReadMessage()
			if err != nil {
				break
			}

			if messageType == websocket.TextMessage {
				err := s.handlerMessage(socket, string(message))
				if err != nil {
					return
				}
			}
		}
	} else if strings.HasPrefix(r.URL.Path, "/socket.io/") {
		fileName := strings.Replace(r.URL.Path, "/socket.io/", "", 1)
		if fileName == "" {
			if transport == "polling" {
				switch r.Method {
				case http.MethodGet:
					if sid == "" {
						s.handleHandshake(w, r)
						return
					}
					s.handlePoll(w, r)
				case http.MethodPost:
					s.handlePost(w, r)
				default:
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				}
			}
			return
		}
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
			if fileName == "" {
				return c.Next()
			}
			return filesystem.SendFile(c, http.FS(clientDistFs), fileName)
		}
		return fiber.ErrUpgradeRequired
	})
	router.Get("/", s.new())
	router.Post("/", s.fiberHandlerPost())
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

func (s *Io) Use(fn func(socket *Socket, next func() *UseError) *UseError) {
	s.use = fn
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
	return func(ctx *fiber.Ctx) error {
		if ctx.Query("transport") == "websocket" {
			return s.handleWebsocket(ctx)
		} else if ctx.Query("transport") == "polling" {
			if ctx.Query("sid") == "" {
				return adaptor.HTTPHandlerFunc(s.handleHandshake)(ctx)
			}
			return adaptor.HTTPHandlerFunc(s.handlePoll)(ctx)
		} else {
			return ctx.SendStatus(404)
		}
	}
}

func (s *Io) fiberHandlerPost() func(ctx *fiber.Ctx) error {
	return func(ctx *fiber.Ctx) error {
		return adaptor.HTTPHandlerFunc(s.handlePost)(ctx)
	}
}

func (s *Io) handleWebsocket(ctx *fiber.Ctx) error {
	return websocket.New(func(c *websocket.Conn) {
		var socket *Socket
		var err error
		if c.Query("sid") != "" {
			socket, err = s.sockets.get(c.Query("sid"))
			if err != nil {
				ctx.Status(http.StatusBadRequest).SendString("unknown session")
				return
			}
			socket.Conn.fasthttp = c
			defer socket.disconnect()
		} else {
			socket = &Socket{
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
			s.sockets.set(socket)

			socket.engineWrite(engineio.OPEN, engineio.ConnParameters{
				SID:          socket.Id,
				PingInterval: s.pingInterval,
				PingTimeout:  s.pingTimeout,
				MaxPayload:   s.maxPayload,
				Upgrades:     []string{},
			}.ToJson())
		}

		for {
			messageType, message, err := c.ReadMessage()
			if err != nil {
				// if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// }
				return
			}

			if messageType == websocket.TextMessage {
				err := s.handlerMessage(socket, string(message))
				if err != nil {
					return
				}
			}
		}
	})(ctx)
}

func (s *Io) handleHandshake(w http.ResponseWriter, r *http.Request) {
	socket := &Socket{
		Id:  s.randomUUID(),
		Nps: "/",
		Conn: &Conn{
			polling: &protocol.Polling{
				Ready: make(chan struct{}),
			},
		},
		listeners: listeners{
			list: make(map[string][]eventCallback),
		},
		pingTime: s.pingInterval,
		Handshake: engineio.Handshake{
			Headers: r.Header,
			URL:     r.RequestURI,
		},
	}
	socket.dispose = append(socket.dispose, func() {
		s.sockets.delete(socket.Id)
	})
	s.sockets.set(socket)

	socket.engineWrite(engineio.OPEN, engineio.ConnParameters{
		SID:          socket.Id,
		PingInterval: s.pingInterval,
		PingTimeout:  s.pingTimeout,
		MaxPayload:   s.maxPayload,
		Upgrades:     []string{"websocket"},
	}.ToJson())
	socket.Conn.polling.Flush(w)
}

func (s *Io) handlePost(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("EIO") != "4" {
		http.Error(w, "unsupported EIO version", http.StatusBadRequest)
		return
	}
	if q.Get("transport") != "polling" {
		http.Error(w, "unsupported transport", http.StatusBadRequest)
		return
	}

	sid := q.Get("sid")
	if sid == "" {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		fmt.Fprint(w, "ok")
		return
	}

	socket, err := s.sockets.get(sid)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		fmt.Fprint(w, "ok")
		return
	}

	body := make([]byte, r.ContentLength)
	r.Body.Read(body)
	r.Body.Close()

	Separator := "\x1e"
	payload := string(body)
	packets := strings.Split(payload, Separator)

	for _, pkt := range packets {
		if len(pkt) == 0 {
			continue
		}
		err := s.handlerMessage(socket, pkt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	fmt.Fprint(w, "ok")
}

func (s *Io) handlePoll(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("EIO") != "4" {
		http.Error(w, "unsupported EIO version", http.StatusBadRequest)
		return
	}
	if q.Get("transport") != "polling" {
		http.Error(w, "unsupported transport", http.StatusBadRequest)
		return
	}

	sid := q.Get("sid")
	if sid == "" {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		fmt.Fprint(w, engineio.NOOP.String())
		return
	}

	socket, err := s.sockets.get(sid)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		fmt.Fprint(w, engineio.NOOP.String())
		return
	}

	// Flush ngay nếu đã có data
	err = socket.Conn.polling.Flush(w)
	if err == nil {
		return
	}

	// Không có data → chờ (long-poll)
	timeout := time.NewTimer(s.pingInterval)
	defer timeout.Stop()

	select {
	case <-socket.Conn.polling.Ready:
		if socket.Conn != nil {
			socket.Conn.polling.Flush(w)
		}
	case <-timeout.C:
		if socket.Conn != nil {
			socket.engineWrite(engineio.NOOP)
			socket.Conn.polling.Flush(w)
		}
	case <-r.Context().Done():
		socket.disconnect()
	}
}

func (s *Io) handlerMessage(socket *Socket, message string) error {
	enginePacketType := string(message[0:1])
	anyAfterPacketType := string(message[1:])
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

		if special1 > special2 && special2 != -1 {
			special1 = -1
		}

		if special2 > special1 && special1 != -1 {
			special2 = -1
		}

		offsetSpecial3 := 0
		for {
			nextSpecial3 := strings.Index(string(nextMess), ",")
			if nextSpecial3 != -1 {
				nextSpecial3 += offsetSpecial3
			}
			if nextSpecial3 == -1 || (special1 != -1 && nextSpecial3 > special1) || (special2 != -1 && nextSpecial3 > special2) {
				break
			}
			nextMess = nextMess[nextSpecial3+1:]
			offsetSpecial3 += nextSpecial3
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
		case protocol.DISCONNECT.String():
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
		case protocol.CONNECT.String():
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
					socket_nps.writer(protocol.CONNECT_ERROR, map[string]interface{}{
						"message": "Invalid namespace",
					})
					// continue
					return nil
				}
			}

			if s.use != nil {
				err := s.use(socket_nps, func() *UseError {
					return nil
				})
				var useError *UseError
				if errors.As(err, &useError) {
					socket_nps.writer(protocol.CONNECT_ERROR, map[string]interface{}{
						"message": useError.Message,
						"data":    useError.Data,
					})
					// continue
					return nil
				}
			}

			if s.onAuthentication != nil {
				dataJson := map[string]string{}
				json.Unmarshal([]byte(rawpayload), &dataJson)
				if !s.onAuthentication(dataJson) {
					socket_nps.writer(protocol.CONNECT_ERROR, map[string]interface{}{
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
			socket_nps.currentNamespace = func() *Namespace {
				return s.Of(namespace)
			}

			socket_nps.writer(protocol.CONNECT, engineio.ConnParameters{
				SID: socket.Id,
			}.ToJson())

			for _, callback := range s.Of(namespace).onConnection.get("connection") {
				callback(socket_nps)
			}
		case protocol.EVENT.String():
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
			// case protocol.BINARY_EVENT.String():
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
			// case protocol.BINARY_ACK.String():
		}
	case engineio.PING.String():
		socket.engineWrite(engineio.PONG, map[string]interface{}{
			"raw": anyAfterPacketType,
		})

		if socket.Conn != nil &&
			socket.Conn.polling != nil {
			socket.Conn.polling.Close()
		}
	case engineio.CLOSE.String():
		if socket.Conn != nil &&
			socket.Conn.polling != nil {
			socket.Conn.polling.Close()
		}
	}
	return nil
}
