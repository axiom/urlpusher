/*
This package implements a server that will push URL to clients connected via
WebSockets.

It's intended to be used to control several big screens used for monitoring
purposes. The server will push out different URL to various statistics, or
to cute kitties.
*/
package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"
)

type Type string

const (
	TYPE_URL    = "url"
	TYPE_RELOAD = "reload"
	TYPE_ADD    = "add"
	TYPE_DELETE = "delete"

	DIRECTORY_FILE = "directory.json"
)

type Message struct {
	Type    Type   `json:"type"`
	Payload string `json:"payload"`
}

// An URLEntry represents is used to associate an URL with the duration it
// should be shown on the screen (i.e. until the next URL will be pushed).
type URLEntry struct {
	URL      string        `json:"url"`
	Duration time.Duration `json:"duration"`
}

// Container used when decoding a JSON file containing a list of URL entries.
type urlEntries struct {
	Entries []URLEntry `json:"entries"`
}

// An URLDirectory holds a list of URLEntries so that the server can push each
// URL in it out to clients in a timely manner.
type URLDirectory struct {
	directory []URLEntry
	current   int
}

func (d URLDirectory) Current() (URLEntry, bool) {
	if len(d.directory) == 0 {
		return URLEntry{}, false
	}

	return d.directory[d.current], true
}

func (d *URLDirectory) Next() (URLEntry, bool) {
	if len(d.directory) == 0 {
		return URLEntry{}, false
	}

	// Advance the current index
	d.current = (d.current + 1) % len(d.directory)

	return d.directory[d.current], true
}

// Create an URL directory by reading JSON from the provided reader.
func ReadDirectory(r io.Reader) (URLDirectory, error) {
	decoder := json.NewDecoder(r)

	// First we need to read the outer most attribute, which will contain
	// a list of URL entries.
	var entries urlEntries
	if err := decoder.Decode(&entries); err != nil {
		log.Fatal("Could not read JSON", err)
	}

	return URLDirectory{
		directory: entries.Entries,
	}, nil
}

func urlEntry2message(entry URLEntry) Message {
	return Message{Type: TYPE_URL, Payload: entry.URL}
}

type Minion struct {
	websocket *websocket.Conn
	done      chan bool
	outgoing  chan Message
}

func MakeMinion(ws *websocket.Conn) Minion {
	return Minion{
		websocket: ws,
		done:      make(chan bool),
		outgoing:  make(chan Message, 5),
	}
}

type Hub struct {
	minions    map[*Minion]bool
	directory  URLDirectory
	broadcast  chan Message
	incoming   chan Message
	register   chan *Minion
	unregister chan *Minion
	done       chan bool
}

func MakeHub() Hub {
	return Hub{
		minions:    make(map[*Minion]bool),
		directory:  URLDirectory{},
		broadcast:  make(chan Message, 5),
		incoming:   make(chan Message, 5),
		register:   make(chan *Minion),
		unregister: make(chan *Minion),
		done:       make(chan bool),
	}
}

func (h *Hub) Connect(minion *Minion) {
	h.minions[minion] = true
	log.Println("Added minion to hub")

	// Reader
	go func(hub *Hub, minion *Minion) {
		for {
			log.Println("Waiting for incoming websocket message")
			var message Message
			err := websocket.JSON.Receive(minion.websocket, &message)
			if err != nil {
				log.Println("Got bad incoming websocket message", err)
				break
			}
			log.Println("Putting incoming websocket message on incoming chan")
			h.incoming <- message
		}
	}(h, minion)

	// Writer
	go func(hub *Hub, minion *Minion) {
		log.Println("Waiting for outgoing websocket message")
		for message := range minion.outgoing {
			err := websocket.JSON.Send(minion.websocket, message)
			if err != nil {
				log.Println("Failed sending websocket message", err)
				break
			}
		}
	}(h, minion)
}

func (h *Hub) Disconnect(minion *Minion) {
}

func (h Hub) Broadcast(m Message) {
	for minion, _ := range h.minions {
		minion.Send(m)
	}
}

func (hub *Hub) ReadDirectoryFromFile(filename string) (err error) {
	file, err := os.Open("directory.json")
	if err != nil {
		log.Println("Could not open directory file for reading")
		return
	}
	directory, err := ReadDirectory(file)
	if err != nil {
		return
	}
	log.Println(directory)
	hub.directory = directory
	return
}

// Try to send a message to a minion. If it fails we will remove the minion
// from the hub's collection of minions and not try to send any more messages
// to it.
func (minion Minion) Send(message Message) {
	// minion.outgoing <- message
	err := websocket.JSON.Send(minion.websocket, message)
	if err != nil {
		minion.done <- true
	}
}

func (hub *Hub) run() {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case minion := <-hub.register:
			log.Println("Registering websocket connection")
			hub.Connect(minion)
			if entry, ok := hub.directory.Current(); ok {
				message := urlEntry2message(entry)
				minion.Send(message)
			}
		case minion := <-hub.unregister:
			log.Println("Unregistering minion")
			delete(hub.minions, minion)
			close(minion.outgoing)
		case message := <-hub.broadcast:
			log.Println("Broadcasting message", message)
			hub.Broadcast(message)
		case message := <-hub.incoming:
			log.Println("Got incoming message", message)
			switch message.Type {
			case TYPE_RELOAD:
				hub.ReadDirectoryFromFile(DIRECTORY_FILE)
			case TYPE_ADD:
				// Add an URL entry to the directory
			case TYPE_DELETE:
				// Delete an URL entry from the directory
			default:
				log.Println(message.Type)
			}

		// The ticker takes care of broadcasting the next URL in a
		// timely fashion.
		case <-ticker.C:
			log.Println("Ticker")

			// Create a message from an URL entry
			if entry, ok := hub.directory.Next(); ok {
				message := urlEntry2message(entry)
				hub.broadcast <- message

				ticker.Stop()
				ticker = time.NewTicker(entry.Duration)
			}
		}
	}
	ticker.Stop()
}

////////////////////////////////////////////////////////////////////////////////
// Websocet stuffs
////////////////////////////////////////////////////////////////////////////////

func makePusher() (*Hub, func(*websocket.Conn)) {
	hub := MakeHub()
	hub.ReadDirectoryFromFile(DIRECTORY_FILE)

	return &hub, func(ws *websocket.Conn) {
		log.Println("Received incoming websocket connection")
		minion := MakeMinion(ws)
		hub.register <- &minion
		defer func() {
			hub.unregister <- &minion
		}()

		// Wait until we are done with this connection before we
		// release (close) it.
		<-minion.done
	}
}

func htmlHandler(c http.ResponseWriter, req *http.Request) {
	htmlTemplate.Execute(c, req.Host)
}

var htmlTemplate = template.Must(template.ParseFiles("pusher.html"))

func main() {
	hub, pusherHandle := makePusher()

	go func(hub *Hub) {
		log.Println("Starting run loop in go routine")
		hub.run()
	}(hub)

	log.Println("Setting up handlers")
	http.Handle("/pusher", websocket.Handler(pusherHandle))
	http.Handle("/", http.FileServer(http.Dir(".")))

	log.Println("Starting server")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
