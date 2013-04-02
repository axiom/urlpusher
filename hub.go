package urlpusher

// The hub is the central piece that is "shared".
type Hubber interface {
	Run()

	// We want to be able to add new and update existing URL entries
	Set(urlEntry URLEntry)

	// We want to be able to delete URL entries
	Delete(id string)

	// We want to be able to get a listing of the current URL entries
	List(chan []URLEntry)

	// We want to be able to change the order of URL entries
	Order([]string)

	// We want to be able to register minions
	RegisterMinion(chan string)
}

type URLEntry struct {
}

type directory struct {
	entries  map[string]URLEntry
	ordering []string
}

func newDirectory() directory {
	return directory{
		entries: make(map[string]URLEntry),
		ordering: make([]string, 0),
	}
}

type hub struct {
	directory directory
	minions   []chan string
}

func newHub() hub {
	return hub{
		directory: newDirectory(),
		minions: make([]chan string, 0, 5),
	}
}

func (h *hub) RegisterMinion(sink chan string) {
}

func (h *hub) Set(urlEntry URLEntry) {
}

func (h *hub) Delete(id string) {
}

func (h *hub) Order(ids []string) {
}

func (h *hub) Run() {
}

var thehub = hub{}
