package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/IvanProdaiko94/raft-protocol-implementation/env"
	"github.com/IvanProdaiko94/raft-protocol-implementation/rpc"
	"github.com/IvanProdaiko94/raft-protocol-implementation/schema"
	"io/ioutil"
	"math/rand"
	"net/http"
)

func randRange(min, max int) int {
	return rand.Intn(max-min) + min
}

type Server interface {
	Start(ctx context.Context) error
	Stop() error
	ServeHTTP(http.ResponseWriter, *http.Request)
}

type server struct {
	srv     *http.Server
	clients []rpc.Client
	mux     *http.ServeMux
	leader  int
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

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var cmd schema.Command

	body, _ := ioutil.ReadAll(r.Body)
	err := json.Unmarshal(body, &cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if s.leader == -1 {
		s.leader = randRange(0, len(env.Cluster)-1)
	}

	client := s.clients[s.leader]
	resp, err := client.NewEntry(context.Background(), &cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Printf("\nexpected leader: %d. actual: %d\n", s.leader, resp.Leader)

	if resp.Leader != int32(s.leader) {
		s.leader = int(resp.Leader)
		client := s.clients[s.leader]
		resp, err = client.NewEntry(context.Background(), &cmd)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	fmt.Printf("\nleader id: %d. Result: %#v\n", s.leader, resp)

	s.leader = int(resp.Leader)
	_, err = w.Write([]byte(fmt.Sprintf("%t", resp.Success)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func NewServer(addr string, cluster []env.Node) Server {
	mux := http.NewServeMux()
	srv := &server{
		srv:     &http.Server{Addr: addr, Handler: mux},
		clients: rpc.NewClients(cluster),
		mux:     mux,
		leader:  -1,
	}
	mux.Handle("/append", srv)
	return srv
}
