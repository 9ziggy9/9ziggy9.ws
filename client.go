package main

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	// "time"
	"nhooyr.io/websocket"
)

type wsMsg struct {
	msg_t      websocket.MessageType;
	raw        []byte;
	emitter_id uint64;
}

type wsSession struct {
	conn *websocket.Conn;
	ctx   context.Context;
}

// BEGIN: client methods
type wsClient struct {
	id       uint64;
	session *wsSession;
	channel  chan wsMsg;
}
func (me *wsClient) emitMsgs() {
	for {
		select {
		case msg := <-me.channel:
			err := me.session.conn.Write(me.session.ctx, msg.msg_t, msg.raw)
			if err != nil {
				ServerLog(
					ERROR, "failed to send message to client %d:\n  -> %v",
					me.id, err,
				)
				return
			}
		case <-me.session.ctx.Done(): return
		}
	}
}
func (me *wsClient) connect(session *wsSession, rm *wsRoom) error {
	for {
		select {
		case <-session.ctx.Done():
			return nil
		default:	
			msg_t, msg, err := session.conn.Read(session.ctx)
			if err != nil { return err }
			rm.broadcast(wsMsg{msg_t, msg, me.id})
			ServerLog(INFO, "msg :: (client: %d) : %s", me.id, msg[:len(msg) - 1])
		}
	}
}
// END: client methods

// BEGIN: room methods
type wsRoom struct {
	clientCount	 uint64;
	clients			 map[uint64]*wsClient;
	mtx					 sync.RWMutex;
}
func (rm *wsRoom) addClient(session *wsSession) *wsClient {
	rm.mtx.Lock()
	defer rm.mtx.Unlock()

	newClient := &wsClient{ rm.clientCount, session, make(chan wsMsg) }
	rm.clients[newClient.id] = newClient

	ServerLog(SUCCESS, "client %d connected", newClient.id)

	rm.clientCount++
	return newClient
}
func (rm *wsRoom) removeClient(clientId uint64) {
	rm.mtx.Lock()
	defer rm.mtx.Unlock()
	delete(rm.clients, clientId)
	rm.clientCount--
}
func (rm *wsRoom) broadcast(msg wsMsg) {
	for _, client := range rm.clients {
		if (!(msg.emitter_id == client.id)) { client.channel <- msg }
	}
}
// END: room methods

// BEGIN: room provider
type wsRoomProvider struct {
	rooms map[uint64] *wsRoom;
}

func (prov *wsRoomProvider) roomExists(rmId uint64) bool {
	_, ok := prov.rooms[rmId]
	return ok
}
func (prov *wsRoomProvider) createRoom(rmId uint64) *wsRoom {
	prov.rooms[rmId] = &wsRoom{
		clientCount: 0,
		clients: make(map[uint64] *wsClient),
	}
	return prov.rooms[rmId]
}
// END:   room provider

func parseRoomIdFromPath(r *http.Request) uint64 {
	rmId, err := strconv.ParseUint(r.PathValue("rmId"), 10, 64)
	if err != nil {
		ServerLog(ERROR, "invalid room ID: %v", err)
		return 999
	}
	return rmId
}

func routesWS() *http.ServeMux {
	mux := http.NewServeMux()
	var WS_ROOMS = &wsRoomProvider { rooms: make(map[uint64] *wsRoom) }

	mux.HandleFunc("GET /{rmId}", func(w http.ResponseWriter, r *http.Request) {
		ServerLog(INFO, "client attempting to connect")
		rmId := parseRoomIdFromPath(r)

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})

		if err != nil {
			ServerLog(ERROR, "failed to accept WS connection:\n  -> %v", err)
		}
		defer conn.CloseNow()

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		if !WS_ROOMS.roomExists(rmId)  { WS_ROOMS.createRoom(rmId) }

		client := WS_ROOMS.rooms[rmId].addClient(&wsSession{conn, ctx})

		var wg sync.WaitGroup
		wg.Add(2)
			go func() {
				defer wg.Done()
				defer WS_ROOMS.rooms[rmId].removeClient(client.id)
				defer ServerLog(INFO, "disconnecting client %d", client.id)
				client.connect(&wsSession{conn, ctx}, WS_ROOMS.rooms[rmId])
			}()

			go func() { defer wg.Done(); client.emitMsgs() }()
		wg.Wait()

		ServerLog(
			INFO, "current room client count: %d",
			WS_ROOMS.rooms[rmId].clientCount,
		)
		conn.Close(websocket.StatusNormalClosure, "")
	})

	return mux
}
