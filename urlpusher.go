package main

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"net/http"
	"os"
	"text/template"
)

type Message string

type Tags []string

type Minion struct {
	tags      Tags
	websocket *websocket.Conn
	done      chan bool
	outgoing  chan Message
}

func MakeMinion(ws *websocket.Conn) Minion {
	return Minion{
		tags:      []string{},
		websocket: ws,
		done:      make(chan bool),
		outgoing:  make(chan Message, 256),
	}
}

type Hub struct {
	minions    map[*Minion]bool
	broadcast  chan Message
	incoming   chan Message
	register   chan *websocket.Conn
	unregister chan *Minion
	done       chan bool
}

func MakeHub() Hub {
	return Hub{
		minions:    make(map[*Minion]bool),
		broadcast:  make(chan Message),
		incoming:   make(chan Message),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *Minion),
		done:       make(chan bool),
	}
}

func (h *Hub) Connect(minion *Minion) {
	h.minions[minion] = true
	fmt.Println("Added minion to hub")

	go func(hub *Hub, minion *Minion) {
		for {
			fmt.Println("Waiting for incoming websocket message")
			var message Message
			err := websocket.Message.Receive(minion.websocket, &message)
			if err != nil {
				fmt.Println("Got bad incoming websocket message", err)
				break
			}
			fmt.Println("Putting incoming websocket message on incoming chan")
			h.incoming <- message
		}
	}(h, minion)

	go func(hub *Hub, minion *Minion) {
		fmt.Println("Waiting for outgoing websocket message")
		for message := range minion.outgoing {
			err := websocket.Message.Send(minion.websocket, message)
			if err != nil {
				fmt.Println("Failed sending websocket message", err)
				break
			}
		}
	}(h, minion)
}

func (h *Hub) Disconnect(minion *Minion) {
}

func (h Hub) Broadcast(m Message, tags Tags) {
	for minion, _ := range h.minions {
		if minion.Match(tags) {
			minion.Send(m)
		}
	}
}

func (minion Minion) Match(tags Tags) bool {
	return true
}

func (minion Minion) Send(message Message) {
	minion.outgoing <- message
}

func (hub *Hub) run() {
	for {
		fmt.Println("Run loop")
		select {
		case conn := <-hub.register:
			fmt.Println("Registering websocket connection")
			minion := MakeMinion(conn)
			hub.Connect(&minion)
		case minion := <-hub.unregister:
			fmt.Println("Unregistering minion")
			delete(hub.minions, minion)
			close(minion.outgoing)
		case message := <-hub.broadcast:
			fmt.Println("Broadcasting message")
			hub.Broadcast(message, []string{})
		case message := <-hub.incoming:
			fmt.Println("Got incoming message", message)
		}
	}
}

////////////////////////////////////////////////////////////////////////////////
// Websocet stuffs
////////////////////////////////////////////////////////////////////////////////

func makePusher() (*Hub, func(*websocket.Conn)) {
	hub := MakeHub()
	return &hub, func(ws *websocket.Conn) {
		fmt.Println("Received incoming websocket connection")
		hub.register <- ws
		fmt.Println("Registered websocket connection")
	}
}

func htmlHandler(c http.ResponseWriter, req *http.Request) {
	htmlTemplate.Execute(c, req.Host)
}

var htmlTemplate = template.Must(template.ParseFiles("pusher.html"))

func main() {
	hub, pusherHandle := makePusher()

	go func(hub *Hub) {
		fmt.Println("Starting run loop in go routine")
		hub.run()
	}(hub)

	fmt.Println("Setting up handlers")
	http.Handle("/pusher", websocket.Handler(pusherHandle))
	http.HandleFunc("/", htmlHandler)

	fmt.Println("Starting server")
	err := http.ListenAndServe("0.0.0.0:8080", nil)
	if err != nil {
		fmt.Errorf("ListenAndServe: %v\n", err)
		os.Exit(1)
	}

	// Wait to close down...
	fmt.Println("Waiting to close down...")
	select {
	case _ = <-hub.done:
		os.Exit(0)
	}
}
