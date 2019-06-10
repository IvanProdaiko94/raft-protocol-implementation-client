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
	"time"
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
			fmt.Errorf("failed to connect to client: %s", err.Error())
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

	success := false

	ctx, _ := context.WithTimeout(context.Background(), time.Second*2)

	for !success {
		if s.leader == -1 {
			s.leader = randRange(0, len(env.Cluster))
		}

		client := s.clients[s.leader]
		fmt.Printf("\n%#v\n", client)
		resp, err := client.NewEntry(ctx, &cmd)
		if err == context.DeadlineExceeded {
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			continue
		}
		fmt.Printf("\n expected leader: %d. actual: %d\n", s.leader, resp.Leader)
		fmt.Printf("\n leader id: %d. Result: %#v\n", s.leader, resp)

		if int(resp.Leader) != s.leader {
			s.leader = int(resp.Leader)
		}

		if resp.Success {
			success = resp.Success
			_, err = w.Write([]byte(fmt.Sprintf("%t", resp.Success)))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
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
