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
	"github.com/tux21b/gocql/uuid"
	"encoding/json"
	_ "expvar"
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
	TYPE_IMG    = "img"
	TYPE_RELOAD = "reload"
	TYPE_TEXT   = "text"
	TYPE_DELETE = "delete"
	TYPE_LIST   = "list"
	TYPE_SET    = "set"

	FIELD_URL      = "url"
	FIELD_DURATION = "duration"
	FIELD_TYPE     = "type"
)

type Message struct {
	Type    Type
	Payload interface{}
}

// An URLEntry represents is used to associate an URL with the duration it
// should be shown on the screen (i.e. until the next URL will be pushed).
type URLEntry struct {
	ID string
	Name         string
	URL          string
	Type         Type
	Duration     time.Duration
	DumbDuration string
}

func (u URLEntry) Ok() bool {
	return u.URL != "" && u.Type != ""
}

// Container used when decoding a JSON file containing a list of URL entries.
type urlEntries struct {
	Entries []URLEntry
}

// An URLDirectory holds a list of URLEntries so that the server can push each
// URL in it out to clients in a timely manner.
type URLDirectory struct {
	directory map[string]URLEntry
	ordering  []string
	current   int
}

func (d URLDirectory) Current() (URLEntry, bool) {
	if len(d.directory) == 0 {
		return URLEntry{}, false
	}

	return d.directory[d.ordering[d.current]], true
}

func (d *URLDirectory) Next() (entry URLEntry, ok bool) {
	if len(d.ordering) == 0 {
		ok = false
		return
	}

	// Advance the current index
	d.current = (d.current + 1) % len(d.ordering)

	entry, ok = d.directory[d.ordering[d.current]]
	if ok != true {
		return
	}

	if entry.Ok() != true {
		ok = false
		return
	}

	return
}

// Get a slice of sorted URLEntries from the directory
func (d URLDirectory) Entries() (entries []URLEntry) {
	for _, id := range d.ordering {
		entry := d.directory[id]
		entry.DumbDuration = entry.Duration.String()
		entries = append(entries, entry)
	}
	return
}

// Delete an entry with the given id if it exist
func (d *URLDirectory) Delete(id string) {
	if _, ok := d.directory[id]; ok {
		delete(d.directory, id)
		for i, orderId := range d.ordering {
			if orderId == id {
				ordering := d.ordering[0:i]
				d.ordering = append(ordering, d.ordering[i+1:len(d.ordering)]...)
				break
			}
		}
	}
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

	directory := URLDirectory{
		ordering:  make([]string, len(entries.Entries), len(entries.Entries)),
		directory: make(map[string]URLEntry, len(entries.Entries)),
	}

	for i, entry := range entries.Entries {
		directory.ordering[i] = entry.ID
		directory.directory[entry.ID] = entry
	}

	return directory, nil
}

func urlEntry2message(entry URLEntry) Message {
	return Message{Type: entry.Type, Payload: entry.URL}
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
		log.Fatal(err)
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

			case TYPE_TEXT:
				hub.Broadcast(message)

			case TYPE_LIST:
				// Dump the URL directory
				hub.Broadcast(Message{
					Type:    TYPE_LIST,
					Payload: hub.directory.Entries(),
				})

			case TYPE_DELETE:
				// Delete an URL entry from the directory. Payload should be the id
				// of the entry.
				hub.directory.Delete(message.Payload.(string))
				hub.Broadcast(Message{
					Type:    TYPE_LIST,
					Payload: hub.directory.Entries(),
				})

			case TYPE_SET:
				// To some jugling to get an URLEntry instead
				// of generic []interface{}.
				payload, err := json.Marshal(message.Payload)
				if err != nil {
					log.Fatal(err)
				}

				var entry URLEntry
				if err = json.Unmarshal(payload, &entry); err != nil {
					log.Println("Could not parse URLEntry from payload", err)
				}

				// Make sure there always is a name for an entry, even a nonsensical
				// one.
				if entry.ID == "" {
					entry.ID = uuid.RandomUUID().String()
				}

				// Get the entry that we will update.
				directoryEntry, knownEntry := hub.directory.directory[entry.ID]
				if knownEntry == false {
					directoryEntry = entry
				}

				if entry.DumbDuration != "" {
					if duration, err := time.ParseDuration(entry.DumbDuration); err == nil {
						directoryEntry.Duration = duration
					} else {
						log.Printf("Could not parse duration %v. %v", entry.DumbDuration, err)
					}
				}

				if entry.Name != "" {
					directoryEntry.Name = entry.Name
				}

				if entry.URL != "" {
					directoryEntry.URL = entry.URL
				}

				if entry.Type != "" {
					directoryEntry.Type = entry.Type
				}

				hub.directory.directory[entry.ID] = directoryEntry

				if knownEntry == false {
					hub.directory.ordering = append(hub.directory.ordering, directoryEntry.ID)
				}

				log.Printf("%+v", directoryEntry)
				hub.Broadcast(Message{
					Type:    TYPE_LIST,
					Payload: hub.directory.Entries(),
				})

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
			} else {
				log.Println("Skipped over non-ok entry")
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
