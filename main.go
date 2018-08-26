package main

import (
	"log"
	"net/http"

	"github.com/googollee/go-socket.io"
)

// For now, let's just make an example WebRTC setup send an example audio file and focus on making the client work
func main() {
	// Create a websocket server for WebRTC authentication as well as other API functions
	initDB()
	initServer()
}

func initServer() {
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}
	server.On("connection", HandleSocketConnection)
	server.On("error", HandleSocketError)

	http.Handle("/socket.io/", server)
	http.Handle("/", http.FileServer(http.Dir("./stereo-client/")))
	log.Println("Serving at localhost:5000...")
	log.Fatal(http.ListenAndServe(":5000", nil))
}
