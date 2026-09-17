package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"google.golang.org/grpc"

	"sdcc-b1-registry/internal/api"
	"sdcc-b1-registry/internal/config"
	"sdcc-b1-registry/internal/gossip"
	"sdcc-b1-registry/internal/gossippb"
	"sdcc-b1-registry/internal/membership"
	"sdcc-b1-registry/internal/registry"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "percorso del file di configurazione del nodo")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("configurazione non valida: %v", err)
	}

	// cfg.Node.ID e' sia l'OwnerNodeID nel versioning sia l'indirizzo
	// host:grpc_port su cui gli altri nodi raggiungono questo nodo.
	reg := registry.New(cfg.Node.ID)
	mem := membership.New(cfg.Node.ID, cfg.TimeoutSuspect(), cfg.TimeoutDead())
	mem.Seed(cfg.Bootstrap.Seeds)

	gossipSvc := gossip.New(cfg.Node.ID, reg, mem, cfg.Gossip.PeersPerRound, cfg.GossipRoundInterval()/2, cfg.Gossip.DeadCheckEveryNRounds, cfg.ServiceHealthCheckInterval(), cfg.ServiceCheckTimeout())
	gossipSvc.Start(cfg.GossipRoundInterval())

	// Server gRPC: espone Gossip.Exchange agli altri nodi, su una porta
	// separata da quella delle API client-facing.
	grpcAddr := fmt.Sprintf(":%d", cfg.Node.GRPCPort)
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("impossibile aprire il listener gRPC su %s: %v", grpcAddr, err)
	}
	grpcServer := grpc.NewServer()
	gossippb.RegisterGossipServer(grpcServer, gossipSvc)

	go func() {
		log.Printf("registry node %s: gRPC gossip in ascolto su %s", cfg.Node.ID, grpcAddr)
		if err := grpcServer.Serve(grpcLis); err != nil {
			log.Fatalf("server gRPC terminato con errore: %v", err)
		}
	}()

	// Server HTTP: espone le API client-facing Register/Deregister/Discover/List.
	// ListenAndServe blocca per sempre, tenendo vivo il processo principale.
	mux := api.NewMux(reg)
	httpAddr := fmt.Sprintf(":%d", cfg.Node.ListenPort)
	log.Printf("registry node %s: API HTTP in ascolto su %s", cfg.Node.ID, httpAddr)
	log.Fatal(http.ListenAndServe(httpAddr, mux))
}
