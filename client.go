package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"nhooyr.io/websocket"
)

type wsMsg struct {
	msg_t      websocket.MessageType;
	raw        []byte;
	emitter_id uint64;
}

type wsSession struct {
	conn     *websocket.Conn;
	ctx       context.Context;
	cancelCtx context.CancelFunc
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
		case <-me.session.ctx.Done(): return
		case msg := <-me.channel:
			err := me.session.conn.Write(me.session.ctx, msg.msg_t, msg.raw)
			if err != nil {
				ServerLog(
					INFO, "failed to send message to client %d:\n  -> %v",
					me.id, err,
				)
				return
			}
		}
	}
}
func (me *wsClient) connect(
	session *wsSession,
	rm *wsRoom,
) websocket.StatusCode {
	for {
		select {
		case <-session.ctx.Done(): return websocket.StatusNormalClosure
		default:	
			msg_t, msg, err := session.conn.Read(session.ctx)
			if err != nil {
				me.session.cancelCtx()
				return websocket.CloseStatus(err)
			}
			if len(msg) > 0 {
				rm.broadcast(wsMsg{msg_t, msg, me.id})
				ServerLog(INFO, "msg :: (client: %d) : %s", me.id, msg[:len(msg) - 1])
			}
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

	newClient := &wsClient{ rm.assignClientId(), session, make(chan wsMsg) }
	rm.clients[newClient.id] = newClient

	ServerLog(SUCCESS, "client %d connected", newClient.id)

	rm.clientCount++
	return newClient
}
func (rm *wsRoom) removeClient(clientId uint64) {
	defer ServerLog(INFO, "disconnecting client %d", clientId)
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
func (rm *wsRoom) assignClientId() uint64 {
	var clientId uint64 = 0
	for id := range rm.clients {
		if id == clientId { clientId++ }
	}
	return clientId
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
// END: room provider

func parseRoomIdFromPath(r *http.Request) uint64 {
	rmId, err := strconv.ParseUint(r.PathValue("rmId"), 10, 64)
	if err != nil {
		ServerLog(ERROR, "invalid room ID: %v", err)
		return 999
	}
	return rmId
}

var jwtKey = []byte("SUPER_SECRET");

type JwtClaims struct {
	Name string `json:"name"`;
	ID   uint64 `json:"id"`;
	jwt.StandardClaims;
}

type contextKey string
const (
	NameKey contextKey = "name"
	IdKey   contextKey = "name"
	RoleKey contextKey = "role"
)

func validateJWT(tokenString string) (*JwtClaims, error) {
    claims := &JwtClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString, claims,
		func(token *jwt.Token) (interface{}, error) {
        return jwtKey, nil
    })
    if err != nil || !token.Valid {
        return nil, err
    }
    return claims, nil
}

func routesWS(ws_rooms *wsRoomProvider) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{rmId}", func(w http.ResponseWriter, r *http.Request) {
		ServerLog(INFO, "client attempting to connect")
		rmId := parseRoomIdFromPath(r)

		tokenString := r.Header.Get("Authorization")
		if tokenString == "" {
				http.Error(w, "missing token", http.StatusUnauthorized)
				return
		}

		tokenString = strings.TrimPrefix(tokenString, "Bearer ")

		claims, err := validateJWT(tokenString)
		_ = claims;

		if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
		}

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})

		if err != nil {
			ServerLog(INFO, "failed to accept WS connection:\n  -> %v", err)
		}

		ctx, cancelCtx := context.WithCancel(r.Context())

		if !ws_rooms.roomExists(rmId)  { ws_rooms.createRoom(rmId) }

		client := ws_rooms.rooms[rmId].addClient(&wsSession{
			conn, ctx, cancelCtx,
		})

		var wg sync.WaitGroup
		wg.Add(2)
			go func() {
				defer wg.Done()
				defer ws_rooms.rooms[rmId].removeClient(client.id)
				close_status := client.connect(
					&wsSession{conn, ctx, cancelCtx},
					ws_rooms.rooms[rmId],
				);
				ServerLog(INFO, "client disconnection code: %d", close_status)
			}()

			go func() {
				defer wg.Done()
				client.emitMsgs()
			}()
		wg.Wait()

		ServerLog(
			INFO, "current [room: %d] client count: %d",
			rmId, ws_rooms.rooms[rmId].clientCount,
		)
		conn.Close(websocket.StatusNormalClosure, "")
	})

	return mux
}

func keepAlive(ws_rooms *wsRoomProvider) {
	ws_keepalive_ticker := time.NewTicker(1 * time.Second)
	defer ws_keepalive_ticker.Stop()
	for {
		select {
		case <- ws_keepalive_ticker.C:
			for rmId, room := range ws_rooms.rooms {
				for _, client := range room.clients {
					client.session.conn.Ping(client.session.ctx)
					ServerLog(INFO, "PINGING client %d in room %d", client.id, rmId)
				}
			}
		}
	}
}
