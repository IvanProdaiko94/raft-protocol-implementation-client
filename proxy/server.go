package proxy

import (
	"context"
	"github.com/IvanProdaiko94/raft-protocol-implementation/env"
	"github.com/IvanProdaiko94/raft-protocol-implementation/rpc"
	"net/http"
)

type Server interface {
	Start(ctx context.Context) error
	Stop() error
	RegisterHandleFunc(pattern string, handler http.HandlerFunc)
	GetClients() []rpc.Client
}

type server struct {
	srv     *http.Server
	clients []rpc.Client
	mux     *http.ServeMux
}

func (s *server) GetClients() []rpc.Client {
	return s.clients
}

func (s *server) Start(ctx context.Context) error {
	for _, client := range s.clients {
		if err := client.Deal(); err != nil {
			panic(err)
		}
	}
	if err := s.srv.ListenAndServe(); err != nil {
		return err
	}
	return nil
}

func (s *server) Stop() error {
	for _, client := range s.clients {
		client.Close()
	}
	return s.srv.Shutdown(context.Background())
}

func (s *server) RegisterHandleFunc(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
}

func NewServer(addr string, cluster []env.Node) Server {
	mux := http.NewServeMux()
	return &server{
		srv:     &http.Server{Addr: addr, Handler: mux},
		clients: rpc.NewClients(cluster),
		mux:     mux,
	}
}
