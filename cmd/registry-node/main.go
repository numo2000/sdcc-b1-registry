package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sdcc-b1-registry/internal/api"
	"sdcc-b1-registry/internal/config"
	"sdcc-b1-registry/internal/gossip"
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

	// cfg.Node.ID e' sia l'OwnerNodeID usato nel versioning (ARCHITETTURA.md,
	// sezione 5) sia l'indirizzo su cui gli altri nodi raggiungono questo nodo
	// per il gossip (ARCHITETTURA.md, sezione 6) — es. "registry-node-1:7000".
	reg := registry.New(cfg.Node.ID)
	mem := membership.New(cfg.TimeoutSuspect(), cfg.TimeoutDead())
	mem.Seed(cfg.Bootstrap.Seeds)

	gossipSvc := gossip.New(cfg.Node.ID, reg, mem, cfg.Gossip.PeersPerRound, cfg.GossipRoundInterval()/2)

	mux := api.NewMux(reg)
	mux.HandleFunc(gossip.ExchangePath, gossipSvc.ExchangeHandler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gossipSvc.Start(ctx, cfg.GossipRoundInterval())

	addr := fmt.Sprintf(":%d", cfg.Node.ListenPort)
	server := &http.Server{Addr: addr, Handler: mux}

	go func() {
		log.Printf("registry node %s in ascolto su %s", cfg.Node.ID, addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server HTTP terminato con errore: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("arresto in corso...")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
}
