package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func tcpConnect() net.Listener {
	defer ServerLog(SUCCESS, "successfully opened TCP connection")
	tcp_in, err := net.Listen("tcp", ":"+os.Getenv("PORT"))
	if err != nil {
		ServerLog(ERROR, "failed to open TCP connection\n  -> %v", err)
	}
	return tcp_in
}

func routesMain(ws_rooms *wsRoomProvider) *http.ServeMux {
	mux   := http.NewServeMux()
	// files := http.FileServer(http.Dir("static"))
	// mux.Handle("/", http.StripPrefix("/", files))
	mux.Handle("/", routesWS(ws_rooms))
	return mux
}

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
	if err := LoadEnv(ENV_FILE); err != nil {
		ServerLog(ERROR, "failed to load environment variables:\n  -> %v", err)
	}
}

func main() {
	tcp_in := tcpConnect()
	defer tcp_in.Close()
	ws_keepalive_ticker := time.NewTicker(1 * time.Second)
	defer ws_keepalive_ticker.Stop()

	ws_rooms := &wsRoomProvider { rooms: make(map[uint64] *wsRoom) }

	server := &http.Server{
		Handler:      routesMain(ws_rooms),
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}

	err_ch := make(chan error, 1)
	sig_ch := make(chan os.Signal, 1)
	signal.Notify(sig_ch, os.Interrupt)

	go func() {
		err_ch <- server.Serve(tcp_in)
	}()

	go func() {
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
	}()

	select {
	case err := <- err_ch: ServerLog(ERROR, "failed to serve:\n  -> %v", err)
	case <- sig_ch: ServerLog(SUCCESS, "received interrupt signal, goodbye")
	}
}
