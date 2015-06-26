package main

import (
	"bufio"
	"fmt"
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
	subscriptions  map[string][]Client
	newClients     chan Client
	defunctClients chan string
	calls          chan rpcCall
}

type Client struct {
	id      string
	session string
	channel chan string
}

type rpcCall struct {
	id   string
	body string
}

func (c *Clients) subscribe(id string, client Client) {
	list, set := c.subscriptions[id]
	if !set {
		c.subscriptions[id] = []Client{}
	}
	c.subscriptions[id] = append(list, client)
}

func (c *Clients) unsubscribe(id string, client Client) {
	list := c.subscriptions[id]
	for i, other := range list {
		if other == client {
			c.subscriptions[id] = append(list[:i], list[i+1:]...)
			return
		}
	}
}

func (c *Clients) Start(conn net.Conn) {
	for {
		select {
		case client := <-c.newClients:
			c.subscriptions[client.id] = []Client{client}
			c.subscribe(client.session, client)
			c.subscribe("world", client)
			fmt.Fprintf(conn, "connected %s %d\n", client.id, client.session)
			fmt.Printf("-> connected %s %d\n", client.id, client.session)
		case id := <-c.defunctClients:
			client := c.subscriptions[id][0]
			delete(c.subscriptions, id)
			c.unsubscribe(client.session, client)
			c.unsubscribe("world", client)
			fmt.Fprintf(conn, "disconnected %s %d\n", id, time.Now().Unix())
			fmt.Printf("-> disconnected %s %d\n", id, time.Now().Unix())
		case call := <-c.calls:
			fmt.Fprintf(conn, "call %s %s\n", call.id, call.body)
			fmt.Printf("-> call %s %s\n", call.id, call.body)
		}
	}
}

//for _, s := range clients.subscriptions[id] {
//}

func (c *Clients) processCall(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading body.", http.StatusInternalServerError)
		fmt.Println(err)
		return
	}
	c.calls <- rpcCall{r.URL.Path, string(body)}
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
	c.newClients <- Client{id, sessionId, messageChan}

	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		c.defunctClients <- id
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

var pattern = regexp.MustCompile(`(\S+) (\S+) (.+)`)

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
			for _, client := range clients.subscriptions[id] {
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
		make(map[string][]Client),
		make(chan Client),
		make(chan string),
		make(chan rpcCall),
	}

	go clients.Start(conn)

	go ReadCommands(conn, clients)

	http.Handle("/call/", http.StripPrefix("/call/", http.HandlerFunc(clients.processCall)))
	http.Handle("/events/", http.StripPrefix("/events/", http.HandlerFunc(clients.processStream)))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	http.HandleFunc("/sse.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "sse.js")
	})
	panic(http.ListenAndServe(":8000", nil))
}
