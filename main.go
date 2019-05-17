package main

import (
	"context"
	"fmt"
	"github.com/IvanProdaiko94/raft-protocol-implementation-client/proxy"
	"github.com/IvanProdaiko94/raft-protocol-implementation/env"
	"log"
	"os"
	"os/signal"
	"syscall"
)

const addr = "localhost:8080"

func main() {
	srv := proxy.NewServer(addr, env.Cluster)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL)

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
