package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/doquangtan/socketio/v4"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
)

func socketIoHandle(io *socketio.Io) {
	io.OnAuthentication(func(params map[string]string) bool {
		token, ok := params["token"]
		if !ok || token != "123" {
			return false
		}
		return true
	})

	io.OnConnection(func(socket *socketio.Socket) {
		println("connect", socket.Nps, socket.Id)
		socket.Join("demo")
		io.To("demo").Emit("test", socket.Id+" join us room...", "server message")

		socket.On("connected", func(event *socketio.EventPayload) {
			for i := 0; i < 100; i++ {
				go func() {
					socket.Emit("chat message", "Main concurrent 1_"+strconv.Itoa(i))
				}()
				go func() {
					socket.Emit("chat message", "Main concurrent 2_"+strconv.Itoa(i))
				}()
				go func() {
					socket.Emit("chat message", "Main concurrent 3_"+strconv.Itoa(i))
				}()
				go func() {
					socket.Emit("chat message", "Main concurrent 4_"+strconv.Itoa(i))
				}()
				go func() {
					socket.Emit("chat message", "Main concurrent 5_"+strconv.Itoa(i))
				}()
			}
		})
		socket.On("test", func(event *socketio.EventPayload) {
			log.Println("Test: ", event.Data)
			socket.Emit("test", event.Data...)
		})

		socket.On("join-room", func(event *socketio.EventPayload) {
			if len(event.Data) > 0 && event.Data[0] != nil {
				socket.Join(event.Data[0].(string))
			}
		})

		socket.On("to-room", func(event *socketio.EventPayload) {
			socket.To("demo").To("demo2").Emit("test", "hello")
		})

		socket.On("leave-room", func(event *socketio.EventPayload) {
			socket.Leave("demo")
			socket.Join("demo2")
		})

		socket.On("my-room", func(event *socketio.EventPayload) {
			socket.Emit("my-room", socket.Rooms())
		})

		socket.On("chat message", func(event *socketio.EventPayload) {
			socket.Emit("chat message", event.Data[0])

			if len(event.Data) > 2 {
				log.Println(socket.Nps, ": ", event.Data[2].(map[string]interface{}))
			}

			if event.Ack != nil {
				event.Ack("hello from name space root", map[string]interface{}{
					"Test": "ok",
				})
			}
		})

		socket.On("disconnecting", func(event *socketio.EventPayload) {
			println("disconnecting", socket.Nps, socket.Id)
		})

		socket.On("disconnect", func(event *socketio.EventPayload) {
			println("disconnect", socket.Nps, socket.Id)
		})
	})

	io.Of("/hello").OnConnection(func(socket *socketio.Socket) {
		println("connect", socket.Nps, socket.Id)

		socket.On("chat message", func(event *socketio.EventPayload) {
			socket.Emit("chat message", event.Data[0])

			if len(event.Data) > 2 {
				log.Println(socket.Nps, ": ", event.Data[2].(map[string]interface{}))
			}

			if event.Ack != nil {
				event.Ack("hello from nps test", map[string]interface{}{
					"Test": "ok",
				})
			}
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
	app.Listen(":3300")
}

func usingWithGin() {
	io := socketio.New()
	socketIoHandle(io)

	router := gin.Default()
	router.Use(static.Serve("/", static.LocalFile("./public", false)))
	router.GET("/socket.io/*any", gin.WrapH(io.HttpHandler()))
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
	// httpServer()
	httpServerWithCors()
	// usingWithGin()

	// socketClientTest()

	// usingWithGoFiber()

}

func socketClientTest() {
	socket := socketio.Connect()
	log.Println(socket)
}
