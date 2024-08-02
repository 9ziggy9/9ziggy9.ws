package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
	"nhooyr.io/websocket"
)

func tcpConnect() net.Listener {
	defer ServerLog(SUCCESS, "successfully opened TCP connection")
	tcp_in, err := net.Listen("tcp", ":"+os.Getenv("PORT"))
	if err != nil {
		ServerLog(ERROR, "failed to open TCP connection\n  -> %v", err)
	}
	return tcp_in
}

func handleWSJoin(ctx context.Context, conn *websocket.Conn) {
	defer conn.Close(websocket.StatusNormalClosure, "connection closed")
	for {
		mtype, msg, err := conn.Read(ctx)
		if err != nil { ServerLog(ERROR, "error reading message:\n  -> %v", err) }

		ServerLog(INFO, "message received: %s", msg)

		err = conn.Write(ctx, mtype, msg)
		if err != nil { ServerLog(ERROR, "error writing message:\n  -> %v", err) }
	}
}

func routesWS() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/join", func(w http.ResponseWriter, r *http.Request) {
		ServerLog(INFO, "someone joining socket")
		ctx := r.Context()
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			ServerLog(ERROR, "failed to accept WS connection:\n  -> %v", err)
		}
		go handleWSJoin(ctx, conn)
	})
	mux.HandleFunc("/ws/send", func(w http.ResponseWriter, r *http.Request) {
		ServerLog(INFO, "received request on /ws/send: %v", r)
	})
	return mux
}

func routesMain() *http.ServeMux {
	mux   := http.NewServeMux()
	files := http.FileServer(http.Dir("static"))
	mux.Handle("/", http.StripPrefix("/", files))
	mux.Handle("/ws/", routesWS())
	return mux
}

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC | log.Llongfile)
	if err := LoadEnv(ENV_FILE); err != nil {
		ServerLog(ERROR, "failed to load environment variables:\n  -> %v", err)
	}
}


func main() {
	tcp_in := tcpConnect()
	defer tcp_in.Close()

	server := &http.Server{
		Handler:      routesMain(),
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}

	err_ch := make(chan error, 1)
	sig_ch := make(chan os.Signal, 1)
	signal.Notify(sig_ch, os.Interrupt)

	go func() {
		err_ch <- server.Serve(tcp_in)
	}()

	select {
	case err := <-err_ch: ServerLog(ERROR, "failed to serve:\n  -> %v", err)
	case <-sig_ch: ServerLog(SUCCESS, "received interrupt signal, goodbye")
	}
}
