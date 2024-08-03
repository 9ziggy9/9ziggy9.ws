package main

import (
	"context"
	"net/http"
	"sync"
	"time"
	"nhooyr.io/websocket"
)

type wsMsg struct {
	msg_t      websocket.MessageType;
	raw        []byte;
	emitter_id uint64;
}

type wsSession struct {
	conn *websocket.Conn;
	ctx *context.Context;
}

// BEGIN: client methods
type wsClient struct {
	id      uint64;
	session *wsSession;
	channel chan wsMsg;
}
func (me *wsClient) emitMsgs() {
	for {
		msg := <-me.channel
		err := me.session.conn.Write(*me.session.ctx, msg.msg_t, msg.raw)
		if err != nil { break }
	}
}
func (me *wsClient) connect(session *wsSession, rm *wsRoom) {
	for {
		msg_t, msg, err := session.conn.Read(*session.ctx)
		rm.broadcast(wsMsg{msg_t, msg, me.id})
		if err != nil { break }
		ServerLog(INFO, "msg(client: %d) : %s", me.id, msg[:len(msg) - 1])
	}
}
// END: client methods

// BEGIN: room methods
type wsRoom struct {clientCount uint64; clients []*wsClient;}
func (rm *wsRoom) addClient(session *wsSession) *wsClient {
	newClient := &wsClient{ rm.clientCount, session, make(chan wsMsg) }
	rm.clients = append(rm.clients, newClient)
	rm.clientCount++
	return newClient
}
func (rm *wsRoom) broadcast(msg wsMsg) {
	for _, client := range rm.clients {
		if (!(msg.emitter_id == client.id)) { client.channel <- msg }
	}
}
// END: room methods

var WS_ROOM = &wsRoom {
	clientCount: 0,
	clients:     make([]*wsClient, 0),
}

func routesWS() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/ws/join", func(w http.ResponseWriter, r *http.Request) {
		ServerLog(INFO, "client attempting to connect")
		defer ServerLog(INFO, "client disconnected")

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			ServerLog(ERROR, "failed to accept WS connection:\n  -> %v", err)
		}
		defer conn.CloseNow()

		ctx, cancel := context.WithTimeout(r.Context(), time.Minute * 5)
		defer cancel()

		client := WS_ROOM.addClient(&wsSession{conn, &ctx})
		ServerLog(SUCCESS, "client %d connected", client.id)

		var wg sync.WaitGroup
		wg.Add(2)
			go func() { client.connect(&wsSession{conn, &ctx}, WS_ROOM) } ()
			go func() { client.emitMsgs() } ()
		wg.Wait()
	})

	return mux
}
