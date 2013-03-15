/*
Package urlpusher implements a server that will push URLs to clients connected
via WebSockets.

It's intended to be used to control several big screens used for monitoring
purposes. The server will push out different URL to various statistics, or
to cute kitties.
*/
package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type Type string

const (
	TYPE_URL    = "url"
	TYPE_RELOAD = "reload"
	TYPE_ADD    = "add"
	TYPE_TEXT   = "text"
	TYPE_DELETE = "delete"
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
		log.Fatal("Could not decode entries in dictionary file", err)
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
		outgoing:  make(chan Message, 1),
	}
}

type Hubber interface {
	Register(*Minion)
	Unregister(*Minion)
	Broadcast(Message)
}

type hub struct {
	minions    map[*Minion]bool
	directory  URLDirectory
	broadcast  chan Message
	incoming   chan Message
	register   chan *Minion
	unregister chan *Minion
	done       chan bool
}

func MakeHub() hub {
	return hub{
		minions:    make(map[*Minion]bool),
		directory:  URLDirectory{},
		broadcast:  make(chan Message, 1),
		incoming:   make(chan Message, 1),
		register:   make(chan *Minion),
		unregister: make(chan *Minion),
		done:       make(chan bool),
	}
}

func (h *hub) Register(minion *Minion) {
	// Add the minion to the hub
	h.minions[minion] = true

	// Setup an asynchronous reader that will put incoming messages into
	// the hub's incoming chan.
	go func(hub *hub, minion *Minion) {
		for {
			var message Message
			err := websocket.JSON.Receive(minion.websocket, &message)
			if err != nil {
				log.Println("Got bad incoming websocket message", err)
				break
			}
			h.incoming <- message
		}
	}(h, minion)

	// Setup an asynchronous writer that will receive messages on the
	// hub's broadcast chan, and send them to all the registered minions.
	go func(hub *hub, minion *Minion) {
		for message := range minion.outgoing {
			err := websocket.JSON.Send(minion.websocket, message)
			if err != nil {
				log.Println("Failed sending websocket message", err)
				break
			}
		}
	}(h, minion)
}

func (h *hub) Unregister(minion *Minion) {
	delete(h.minions, minion)
	close(minion.outgoing)
}

func (h hub) Broadcast(m Message) {
	for minion, _ := range h.minions {
		minion.Send(m)
	}
}

func (hub *hub) ReadDirectoryFromFile(filename string) (err error) {
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

func (hub *hub) run() {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case minion := <-hub.register:
			log.Println("Registering minion")
			hub.Register(minion)
			if entry, ok := hub.directory.Current(); ok {
				message := urlEntry2message(entry)
				minion.Send(message)
			}
		case minion := <-hub.unregister:
			log.Println("Unregistering minion")
			hub.Unregister(minion)
		case message := <-hub.broadcast:
			log.Println("->", message)
			hub.Broadcast(message)
		case message := <-hub.incoming:
			log.Println("<-", message)
			switch message.Type {
			case TYPE_RELOAD:
				hub.ReadDirectoryFromFile(*directoryFile)
				hub.Broadcast(Message{Type: TYPE_TEXT, Payload: "reloading"})
				hub.Broadcast(Message{Type: TYPE_RELOAD})
			case TYPE_ADD:
				// Add an URL entry to the directory
				duration := 3 * time.Second
				urlEntry := URLEntry{
					URL:      message.Payload,
					Duration: duration,
				}
				hub.directory.directory = append(hub.directory.directory, urlEntry)
				log.Println("Adding URL to directory:", urlEntry)
			case TYPE_DELETE:
				// Delete an URL entry from the directory
			case TYPE_TEXT:
				hub.Broadcast(message)
			default:
				log.Println("Got unknown message type: ", message.Type)
			}

		// The ticker takes care of broadcasting the next URL in a
		// timely fashion.
		case <-ticker.C:
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

func makePusher() (*hub, func(*websocket.Conn)) {
	hub := MakeHub()
	hub.ReadDirectoryFromFile(*directoryFile)

	return &hub, func(ws *websocket.Conn) {
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

var (
	port          = flag.Int("port", 8012, "Port to listen on")
	host          = flag.String("host", "0.0.0.0", "Host to bind to")
	directoryFile = flag.String("directory", "directory.json", "File to read URL entries from")
)

func main() {
	flag.Parse()

	hubert, pusherHandle := makePusher()

	go func(hubert *hub) {
		hubert.run()
	}(hubert)

	http.Handle("/pusher", websocket.Handler(pusherHandle))
	http.Handle("/", http.FileServer(http.Dir(".")))

	log.Printf("Listening on %v:%v\n", *host, *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%v:%v", *host, *port), nil))
}
