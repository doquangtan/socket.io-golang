package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/doquangtan/socketio/v4"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
)

func socketIoHandle(io *socketio.Io) {
	users := &userStore{data: make(map[string]string)}

	io.OnAuthentication(func(params map[string]string) bool {
		token, ok := params["token"]
		if !ok || token != "123" {
			return false
		}
		return true
	})

	io.Use(func(socket *socketio.Socket, next func() *socketio.UseError) *socketio.UseError {
		if socket.Handshake.Headers.Get("Abcd") != "xyzz" {
			return &socketio.UseError{
				Message: "Loi",
				Data: map[string]interface{}{
					"content": "Please retry later",
				},
			}
		}
		return next()
	})

	io.OnConnection(func(socket *socketio.Socket) {
		log.Printf("[%s] Người dùng mới kết nối: %s",
			time.Now().Format("15:04:05"), socket.Id)

		// Sự kiện khi người dùng tham gia chat
		socket.On("join", func(event *socketio.EventPayload) {
			if len(event.Data) == 0 || event.Data[0] == nil {
				return
			}
			username, ok := event.Data[0].(string)
			if !ok {
				return
			}

			users.set(socket.Id, username)
			log.Printf("%s đã tham gia phòng chat", username)

			io.Emit("user-joined", map[string]interface{}{
				"message":   fmt.Sprintf("%s đã tham gia phòng chat", username),
				"users":     users.list(),
				"userCount": users.count(),
			})
		})

		// Sự kiện nhận tin nhắn
		socket.On("send-message", func(event *socketio.EventPayload) {
			if len(event.Data) == 0 || event.Data[0] == nil {
				return
			}
			payload, ok := event.Data[0].(map[string]interface{})
			if !ok {
				return
			}
			text, _ := payload["text"].(string)
			username := users.get(socket.Id)
			ts := time.Now().Format("15:04:05")

			log.Printf("[%s] %s: %s", ts, username, text)

			io.Emit("receive-message", map[string]interface{}{
				"username":  username,
				"text":      text,
				"timestamp": ts,
			})
		})

		// Sự kiện đang gõ
		socket.On("typing", func(event *socketio.EventPayload) {
			socket.Broadcast().Emit("user-typing", map[string]interface{}{
				"username": users.get(socket.Id),
			})
		})

		// Sự kiện ngừng gõ
		socket.On("stop-typing", func(event *socketio.EventPayload) {
			socket.Broadcast().Emit("user-stopped-typing", map[string]interface{}{
				"username": users.get(socket.Id),
			})
		})

		// Sự kiện ngắt kết nối
		socket.On("disconnect", func(event *socketio.EventPayload) {
			username := users.get(socket.Id)
			users.delete(socket.Id)
			log.Printf("%s đã ngắt kết nối", username)

			io.Emit("user-left", map[string]interface{}{
				"message":   fmt.Sprintf("%s đã rời khỏi phòng chat", username),
				"users":     users.list(),
				"userCount": users.count(),
			})
		})
	})
}

func usingWithGoFiber() {
	io := socketio.New()
	socketIoHandle(io)

	app := fiber.New(fiber.Config{})
	app.Static("/", "./public")
	app.Use("/", io.FiberMiddleware)
	app.Route("/socket.io", io.FiberRoute)
	err := app.Listen(":3300")
	log.Fatal(err)
}

func usingWithGin() {
	io := socketio.New()
	socketIoHandle(io)

	router := gin.Default()
	router.Use(static.Serve("/", static.LocalFile("./public", false)))
	router.Any("/socket.io/*any", gin.WrapH(io.HttpHandler()))
	router.Run(":3300")
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*") // Allow all origins
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight OPTIONS request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func httpServerWithCors() {
	io := socketio.New()
	socketIoHandle(io)

	mux := http.NewServeMux()
	corsHandler := corsMiddleware(io.HttpHandler())
	mux.Handle("/socket.io/", corsHandler)
	// http.Handle("/socket.io/", io.HttpHandler())
	mux.Handle("/", http.FileServer(http.Dir("./public")))

	server := &http.Server{
		Addr:    ":3300",
		Handler: mux,
	}

	fmt.Println("Server listenning on port 3300 ...")
	fmt.Println(server.ListenAndServe())
}

func httpServer() {
	io := socketio.New()
	socketIoHandle(io)
	http.Handle("/socket.io/", io.HttpHandler())
	http.Handle("/", http.FileServer(http.Dir("./public")))
	fmt.Println("Server listenning on port 3300 ...")
	fmt.Println(http.ListenAndServe(":3300", nil))
}

func main() {
	httpServer()
	// httpServerWithCors()
	// usingWithGin()

	// socketClientTest()

	// usingWithGoFiber()

}

func socketClientTest() {
	socket := socketio.Connect()
	log.Println(socket)
}
