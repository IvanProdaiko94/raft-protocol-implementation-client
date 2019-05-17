package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/IvanProdaiko94/raft-protocol-implementation-client/proxy"
	"github.com/IvanProdaiko94/raft-protocol-implementation/env"
	"github.com/IvanProdaiko94/raft-protocol-implementation/schema"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

const addr = "localhost:8080"

func randRange(min, max int) int {
	return rand.Intn(max-min) + min
}

func main() {
	srv := proxy.NewServer(addr, env.Cluster)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL)

	var leader = -1

	srv.RegisterHandleFunc("/push", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var cmd schema.Command

		body, _ := ioutil.ReadAll(r.Body)
		err := json.Unmarshal(body, &cmd)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if leader == -1 {
			leader = randRange(0, len(env.Cluster)-1)
		}

		client := srv.GetClients()[leader]
		resp, err := client.Push(context.Background(), &cmd)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if resp.Leader != int32(leader) {
			leader = int(resp.Leader)
			client := srv.GetClients()[leader]
			resp, err = client.Push(context.Background(), &cmd)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		fmt.Printf("Leader id: %d. Result: %#v", leader, resp)

		leader = int(resp.Leader)
		_, err = w.Write([]byte(fmt.Sprintf("%t", resp.Success)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	go func() {
		fmt.Printf("Launched http server on %s\n", addr)
		if err := srv.Start(ctx); err != nil {
			panic(err)
		}
	}()

	<-stop

	err := srv.Stop()
	if err != nil {
		panic(err)
	}

	log.Println("Shutting down http server")
}
