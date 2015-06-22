package main

import (
	"bufio"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"os"
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

type Broker struct {
	idByClient     map[chan string]string
	clientById     map[string]chan string
	newClients     chan chan string
	defunctClients chan chan string
	messages       chan string
}

func (b *Broker) Start() {
	for {
		select {
		case s := <-b.newClients:
			id := randId()
			b.idByClient[s] = id
			b.clientById[id] = s
			fmt.Printf("connected %s\n", id)
		case s := <-b.defunctClients:
			id := b.idByClient[s]
			delete(b.idByClient, s)
			delete(b.clientById, id)
			fmt.Printf("disconnected %s\n", id)
		case msg := <-b.messages:
			for s, _ := range b.idByClient {
				s <- msg
			}
		}
	}
}

func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

var responseTemplate = template.Must(template.ParseFiles("template.html"))

func MainPageHandler(w http.ResponseWriter, r *http.Request) {
	// Did you know Golang's ServeMux matches only the
	// prefix of the request URL?  It's true.  Here we
	// insist the path is just "/".
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	responseTemplate.Execute(w, "")
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	b := &Broker{
		make(map[chan string]string),
		make(map[string]chan string),
		make(chan (chan string)),
		make(chan (chan string)),
		make(chan string),
	}

	go b.Start()
	http.Handle("/events/", b)

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				panic(err)
			}
			b.messages <- line
		}
	}()

	http.Handle("/", http.HandlerFunc(MainPageHandler))
	panic(http.ListenAndServe(":8000", nil))
}
