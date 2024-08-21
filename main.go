package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
	"context"
	"github.com/dgrijalva/jwt-go"
	srv "github.com/9ziggy9/9ziggy9.ws/server"
)

func tcpConnect() net.Listener {
	defer srv.Log(srv.SUCCESS, "successfully opened TCP connection");
	tcp_in, err := net.Listen("tcp", ":"+os.Getenv("PORT"))
	if err != nil {
		srv.Log(srv.ERROR, "failed to open TCP connection\n  -> %v", err)
	}
	return tcp_in
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

func jwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tkn_cookie, err := r.Cookie("token");
		if err != nil {
			srv.Log(srv.ERROR, "missing jwt in request");
			http.Error(w, "missing token", http.StatusUnauthorized);
			return;
		}

		tkn_str  := tkn_cookie.Value;
		claims   := &JwtClaims{};
		tkn, err := jwt.ParseWithClaims(
			tkn_str, claims,
			func(tkn *jwt.Token) (interface{}, error) {
				return jwtKey, nil
			},
		);

		if err != nil || !tkn.Valid {
			srv.Log(srv.ERROR, "invalid jwt in request");
			http.Error(w, "invalid token", http.StatusUnauthorized);
			return;
		}

		ctx := context.WithValue(r.Context(), NameKey, claims.Name);
		ctx  = context.WithValue(ctx, IdKey, claims.ID);
		ctx  = context.WithValue(ctx, RoleKey, "standard");

		next.ServeHTTP(w, r.WithContext(ctx));
	});
}

func routesMain(ws_rooms *wsRoomProvider) *http.ServeMux {
	mux   := http.NewServeMux()
	mux.Handle("/", jwtMiddleware(routesWS(ws_rooms)))
	return mux
}

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
	if err := LoadEnv(ENV_FILE); err != nil {
		srv.Log(srv.ERROR, "failed to load environment variables:\n  -> %v", err)
	}
}

func main() {
	tcp_in := tcpConnect()
	defer tcp_in.Close()

	ws_rooms := &wsRoomProvider { rooms: make(map[uint64] *wsRoom) }

	server := &http.Server{
		Handler:      routesMain(ws_rooms),
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}

	err_ch := make(chan error, 1)
	sig_ch := make(chan os.Signal, 1)
	signal.Notify(sig_ch, os.Interrupt)

	go func() { err_ch <- server.Serve(tcp_in) }()
	go keepAlive(ws_rooms)

	select {
	case err := <- err_ch: srv.Log(srv.ERROR, "failed to serve:\n  -> %v", err)
	case <- sig_ch: srv.Log(srv.SUCCESS, "received interrupt signal, goodbye")
	}
}
