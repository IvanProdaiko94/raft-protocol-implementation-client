package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/IvanProdaiko94/raft-protocol-implementation/env"
	"github.com/IvanProdaiko94/raft-protocol-implementation/rpc"
	"github.com/IvanProdaiko94/raft-protocol-implementation/schema"
	"github.com/golang/protobuf/ptypes/empty"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"sync"
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
	var mu sync.Mutex
	for _, client := range s.clients {
		if err := client.Deal(); err != nil {
			mu.Lock()
			_, _ = os.Stderr.Write([]byte(err.Error()))
			_, _ = os.Stderr.Write([]byte("\n"))
			mu.Unlock()
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

func (s *server) GetLog(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	ctx, _ := context.WithTimeout(context.Background(), time.Second*5)

	for _, cl := range s.clients {
		wg.Add(1)
		go func(client rpc.Client) {
			defer wg.Done()
			log, err := client.GetLog(ctx, &empty.Empty{})

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				_, _ = w.Write([]byte(err.Error()))
				_, _ = w.Write([]byte("\n"))
				return
			}
			data, err := json.Marshal(&log)
			if err != nil {
				_, _ = w.Write([]byte(err.Error()))
				_, _ = w.Write([]byte("\n"))
				return
			}
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n"))
		}(cl)
	}
	wg.Wait()
}

func (s *server) NewEntry(w http.ResponseWriter, r *http.Request) {
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

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	if r.Method == http.MethodGet {
		s.GetLog(w, r)
	}

	if r.Method == http.MethodPost {
		s.NewEntry(w, r)
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
	mux.Handle("/log", srv)
	return srv
}
