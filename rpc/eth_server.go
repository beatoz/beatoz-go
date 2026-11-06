package rpc

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/cors"
)

func newEthRPCHandler(corsOrigins []string) (http.Handler, error) {
	server, err := newEthRPCServer()
	if err != nil {
		return nil, err
	}

	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		server.ServeHTTP(w, r)
	})
	if len(corsOrigins) > 0 {
		handler = cors.New(cors.Options{
			AllowedOrigins: corsOrigins,
			AllowedMethods: []string{http.MethodPost, http.MethodOptions},
			AllowedHeaders: []string{"Content-Type"},
		}).Handler(handler)
	}
	return handler, nil
}

func startEthServer(laddr string, corsOrigins []string) (*http.Server, net.Listener, error) {
	handler, err := newEthRPCHandler(corsOrigins)
	if err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen("tcp", strings.TrimPrefix(laddr, "tcp://"))
	if err != nil {
		return nil, nil, err
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go server.Serve(listener)
	return server, listener, nil
}

func parseEthCORSOrigins(value string) []string {
	var origins []string
	for _, origin := range strings.Split(value, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

var ethServerOnce sync.Once

// R2 has no config.toml integration or graceful shutdown yet. Tendermint RPC must
// be enabled to install tmrpccore's environment; earlier requests hit a recovered panic.
func startEthServerFromEnv() {
	ethServerOnce.Do(func() {
		laddr := os.Getenv("BEATOZ_ETH_RPC_LADDR")
		if laddr == "" {
			return
		}
		_, listener, err := startEthServer(laddr, parseEthCORSOrigins(os.Getenv("BEATOZ_ETH_RPC_CORS")))
		if err != nil {
			panic(fmt.Errorf("eth rpc: listen %s: %w", laddr, err))
		}
		fmt.Fprintf(os.Stderr, "eth rpc: listening on %s\n", listener.Addr())
	})
}
