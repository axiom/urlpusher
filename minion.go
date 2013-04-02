package urlpusher

import (
	"code.google.com/p/go.net/websocket"
)

func HandleMinion(ws *websocket.Conn) {
	push := make(chan string)
	thehub.RegisterMinion(push)

	go func() {
		for {
			if url, ok := <-push; ok {
				websocket.Message.Send(ws, url)
			} else {
				// Chan was closed, we are done
				break
			}
		}
	}()
}
