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
	idByClient     map[chan string]string
	clientById     map[string]chan string
	newClients     chan chan string
	defunctClients chan chan string
}

func (b *Clients) Start() {
	for {
		select {
		case s := <-b.newClients:
			id := randId()
			b.idByClient[s] = id
			b.clientById[id] = s
			fmt.Printf("connected %s %d\n", id, time.Now().Unix())
		case s := <-b.defunctClients:
			id := b.idByClient[s]
			delete(b.idByClient, s)
			delete(b.clientById, id)
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

	messageChan := make(chan string)
	b.newClients <- messageChan

	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		b.defunctClients <- messageChan
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
				for s, _ := range clients.idByClient {
					s <- params
				}
			} else {
				clients.clientById[id] <- params
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
		make(map[chan string]string),
		make(map[string]chan string),
		make(chan (chan string)),
		make(chan (chan string)),
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
