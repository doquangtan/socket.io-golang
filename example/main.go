package main

import (
	"fmt"
	"log"
	"net/http"

	socketio "github.com/doquangtan/socket.io/v4"
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
			socket.Emit("chat message", "Main")
		})
		socket.On("test", func(event *socketio.EventPayload) {
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

	io.Of("/test").OnConnection(func(socket *socketio.Socket) {
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
	app.Use("/", io.Middleware)
	app.Route("/socket.io", io.FiberRoute)
	app.Listen(":3300")
}

func usingWithGin() {
	io := socketio.New()
	socketIoHandle(io)

	router := gin.Default()
	router.Use(static.Serve("/", static.LocalFile("./public", false)))
	router.GET("/socket.io/", gin.WrapH(io.HttpHandler()))
	router.Run(":3300")
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
	// usingWithGoFiber()
	usingWithGin()
}
