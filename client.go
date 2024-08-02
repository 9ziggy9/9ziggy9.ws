package main

import (
	"context"
	"net/http"
	"time"
	"nhooyr.io/websocket"
)

type wsMsg struct {
	msg_t websocket.MessageType;
	raw  []byte;
}

type wsSession struct {
	conn *websocket.Conn;
	ctx *context.Context;
}

type wsClient struct {
	id uint64;
	msgQueue []wsMsg;
	session *wsSession;
}
func (me *wsClient) emitMsgs() {
	for len(me.msgQueue) > 0 {
		var msg wsMsg;
		msg, me.msgQueue = me.msgQueue[0], me.msgQueue[1:]
		me.session.conn.Write(*me.session.ctx, msg.msg_t, msg.raw)
	}
}

type wsChannel struct { clientCount uint64; clients []*wsClient; }
func (ch *wsChannel) addClient(session *wsSession) *wsClient {
	newClient := &wsClient{
		ch.clientCount,
		make([]wsMsg, 0),
		session,
	}
	ch.clients = append(ch.clients, newClient)
	ch.clientCount++
	return newClient
}
func (ch *wsChannel) broadcast(me *wsClient, msg wsMsg) {
	for _, client := range ch.clients {
		if me.id == client.id { continue }
		client.msgQueue = append(client.msgQueue, msg)
	}
}

var WS_CHANNEL = &wsChannel {
	clientCount: 0,
	clients:     make([]*wsClient, 0),
}

func clientConnection(
	ctx context.Context, conn *websocket.Conn, ch *wsChannel,
) {
	client := ch.addClient(&wsSession{conn, &ctx})
	ServerLog(SUCCESS, "client %d connected", client.id)
	for {
		msg_t, msg, err := conn.Read(ctx)
		ch.broadcast(client, wsMsg{msg_t, msg})
		if err != nil { break }
		ServerLog(INFO, "msg(client: %d) : %s", client.id, msg[:len(msg) - 1])
		client.emitMsgs()
	}
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

		clientConnection(ctx, conn, WS_CHANNEL)
	})

	mux.HandleFunc("/ws/send", func(w http.ResponseWriter, r *http.Request) {
		ServerLog(INFO, "received request on /ws/send: %v", r)
	})

	return mux
}
