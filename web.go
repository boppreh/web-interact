package main

import (
	"bufio"
	"fmt"
	"github.com/boppreh/gohandlers"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
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
	clientsById    map[string]Client
	subscriptions  map[string]map[Client]bool
	newClients     chan Client
	defunctClients chan Client
	calls          chan rpcCall
}

type Client struct {
	id      string
	session string
	channel chan string
}

type rpcCall struct {
	client Client
	body   string
}

func (c *Clients) subscribe(id string, client Client) {
	_, set := c.subscriptions[id]
	if !set {
		c.subscriptions[id] = map[Client]bool{client: true}
	} else {
		c.subscriptions[id][client] = true
	}
}

func (c *Clients) unsubscribe(id string, client Client) {
	delete(c.subscriptions[id], client)
	if len(c.subscriptions[id]) == 0 {
		delete(c.subscriptions, id)
	}
}

func (c *Clients) Start(conn net.Conn) {
	for {
		select {
		case client := <-c.newClients:
			c.clientsById[client.id] = client
			c.subscribe(client.id, client)
			c.subscribe(client.session, client)
			c.subscribe("world", client)
			fmt.Fprintf(conn, "connected %s %s\n", client.id, client.session)
			fmt.Printf("-> connected %s %s\n", client.id, client.session)
		case client := <-c.defunctClients:
			c.unsubscribe(client.id, client)
			c.unsubscribe(client.session, client)
			c.unsubscribe("world", client)
			delete(c.clientsById, client.id)
			fmt.Fprintf(conn, "disconnected %s %s\n", client.id, client.session)
			fmt.Printf("-> disconnected %s %s\n", client.id, client.session)
		case call := <-c.calls:
			fmt.Fprintf(conn, "call %s %s\n", call.client.id, call.body)
			fmt.Printf("-> call %s %s\n", call.client.id, call.body)
		}
	}
}

func (c *Clients) processCall(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading body.", http.StatusInternalServerError)
		fmt.Println(err)
		return
	}
	client := c.clientsById[r.URL.Path]
	c.calls <- rpcCall{client, string(body)}
}

func (c *Clients) processStream(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	var sessionId string
	if err == http.ErrNoCookie {
		sessionId = "session" + randId()
		http.SetCookie(w, &http.Cookie{Name: "session",
			Value:    sessionId,
			Path:     "/",
			Expires:  time.Now().AddDate(1, 0, 0),
			MaxAge:   60 * 60 * 24 * 365,
			Secure:   true,
			HttpOnly: true})
	} else {
		sessionId = cookie.Value
	}

	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	id := r.URL.Path
	messageChan := make(chan string)
	client := Client{id, sessionId, messageChan}
	c.newClients <- client

	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		c.defunctClients <- client
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Tell nginx to not buffer. Without this it may take up to a minute
	// for events to arrive at the client.
	w.Header().Set("X-Accel-Buffering", "no")

	// Send something to force the headers to flush.
	fmt.Fprintf(w, "\n\n")
	f.Flush()

	for {
		select {
		case msg := <-messageChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
		case <-time.After(30 * time.Second):
			fmt.Fprintf(w, ":heartbeat\n\n")
		}
		f.Flush()
	}
}

var pattern = regexp.MustCompile(`(\S+) (\S+) (.*)`)

func ReadCommands(conn net.Conn, clients *Clients) {
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			// If the socket is closed we will be having connection errors
			// everywhere when we try to report events. Best to close
			// everything.
			panic("Command center client disconnected.")
		}
		if err != nil {
			panic(err)
		}
		parts := pattern.FindStringSubmatch(line)
		if len(parts) == 0 {
			fmt.Println("Invalid command format. Expected 'command id params'.\n")
			continue
		}
		command := parts[1]
		id := parts[2]
		params := parts[3]
		fmt.Println("<- " + line)
		switch command {
		case "send":
			for client, _ := range clients.subscriptions[id] {
				client.channel <- params
			}
		default:
			fmt.Println("Invalid command " + command + "\n")
		}
	}
}

func waitForClient() net.Conn {
	fmt.Println("Listening for command center client at :8001.")

	ln, err := net.Listen("tcp", ":8001")
	if err != nil {
		panic(err.Error())
	}

	conn, err := ln.Accept()
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Command center client connected.")

	return conn
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	conn := waitForClient()

	clients := &Clients{
		make(map[string]Client),
		make(map[string]map[Client]bool),
		make(chan Client),
		make(chan Client),
		make(chan rpcCall),
	}

	go clients.Start(conn)

	go ReadCommands(conn, clients)

	handlers.ServeIndex("index.html")
	handlers.ServeFile("sse.js")
	handlers.ServeFile("polyfill.js")
	handlers.HandleFuncStripped("/call", clients.processCall)
	handlers.HandleFuncStripped("/events", clients.processStream)
	handlers.Start("8080")
}
