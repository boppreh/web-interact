package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"time"
)

var idChars = []rune("0123456789abcdef")
var idLength = 32

func randId() string {
	b := make([]rune, idLength)
	for i := range b {
		b[i] = idChars[rand.Intn(len(idChars))]
	}
	return string(b)
}

type Clients struct {
	channelById     map[string]chan string
	newClients     chan newClient
	defunctClients chan string
}

type newClient struct {
    id string
    channel chan string
}

func (c *Clients) Start() {
	for {
		select {
		case client := <-c.newClients:
			c.channelById[client.id] = client.channel
			fmt.Printf("connected %s %d\n", client.id, time.Now().Unix())
		case id := <-c.defunctClients:
			delete(c.channelById, id)
			fmt.Printf("disconnected %s %d\n", id, time.Now().Unix())
		}
	}
}

func (b *Clients) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

    id := randId()
	messageChan := make(chan string)
	b.newClients <- newClient{id, messageChan}

	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		b.defunctClients <- id
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Tell nginx to not buffer. Without this it may take up to a minute
	// for events to arrive at the client.
	w.Header().Set("X-Accel-Buffering", "no")

	for {
		msg := <-messageChan
		fmt.Fprintf(w, "data: %s\n\n", msg)
		f.Flush()
	}
}

var pattern = regexp.MustCompile(`(\S+) (\S+) (\S+)`)

func ReadCommands(clients *Clients) {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			panic(err)
		}
		parts := pattern.FindStringSubmatch(line)
		if len(parts) == 0 {
			fmt.Fprintf(os.Stderr, "Invalid command format. Expected 'command id params'.\n")
			continue
		}
		command := parts[1]
		id := parts[2]
		params := parts[3]
		switch command {
		case "send":
			if id == "world" {
				for _, s := range clients.channelById {
					s <- params
				}
			} else {
				clients.channelById[id] <- params
			}
		default:
			fmt.Fprintf(os.Stderr, "Invalid command "+command+"\n")
		}
	}
}

func HandleCall(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading body.", http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, err.Error())
		return
	}
	fmt.Printf("call %s\n", body)
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	clients := &Clients{
		make(map[string]chan string),
		make(chan newClient),
		make(chan string),
	}

	go clients.Start()
	http.Handle("/events/", clients)

	go ReadCommands(clients)

	http.Handle("/call", http.HandlerFunc(HandleCall))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	panic(http.ListenAndServe(":8000", nil))
}
