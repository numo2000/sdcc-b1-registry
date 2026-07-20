// Package gossip implementa il protocollo di anti-entropy push-pull
// (selectPeer, selectToSend, scambio, selectToKeep, processData) per la
// propagazione dello stato del registry tra i nodi, e la failure detection
// basata su timeout sui contatti diretti.
//
// Vedi ARCHITETTURA.md, sezioni 6 e 7.
package gossip

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"sdcc-b1-registry/internal/membership"
	"sdcc-b1-registry/internal/registry"
)

// ExchangePath e' il path HTTP su cui un nodo espone l'endpoint di scambio
// anti-entropy per gli altri nodi.
const ExchangePath = "/internal/gossip/exchange"

// wireEntry e' la rappresentazione sul filo di una registry.ServiceEntry.
type wireEntry struct {
	ServiceID   string            `json:"service_id"`
	Endpoint    string            `json:"endpoint"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Counter     uint64            `json:"counter"`
	OwnerNodeID string            `json:"owner_node_id"`
	Status      string            `json:"status"`
}

func toWire(e registry.ServiceEntry) wireEntry {
	return wireEntry{
		ServiceID:   e.ServiceID,
		Endpoint:    e.Endpoint,
		Metadata:    e.Metadata,
		Counter:     e.Version.Counter,
		OwnerNodeID: e.Version.OwnerNodeID,
		Status:      string(e.Status),
	}
}

func fromWire(w wireEntry) registry.ServiceEntry {
	return registry.ServiceEntry{
		ServiceID: w.ServiceID,
		Endpoint:  w.Endpoint,
		Metadata:  w.Metadata,
		Version:   registry.Version{Counter: w.Counter, OwnerNodeID: w.OwnerNodeID},
		Status:    registry.Status(w.Status),
	}
}

// exchangePayload e' il messaggio scambiato in un round di anti-entropy.
// SenderAddr permette al ricevente di imparare/riconfermare l'indirizzo di
// chi ha iniziato lo scambio, cosi' un nodo appena entrato diventa noto
// anche ai peer che non lo avevano in seed list (join dinamico).
type exchangePayload struct {
	SenderAddr string      `json:"sender_addr"`
	Entries    []wireEntry `json:"entries"`
}

// Service coordina il protocollo di gossip anti-entropy push-pull
// (ARCHITETTURA.md, sezione 6) e la failure detection basata su timeout
// (ARCHITETTURA.md, sezione 7) per un nodo del registry.
type Service struct {
	SelfAddr   string
	Registry   *registry.Registry
	Membership *membership.View
	Client     *http.Client

	PeersPerRound int
	rng           *rand.Rand
}

// New crea un Service di gossip. httpTimeout limita la durata di un singolo
// scambio con un peer, cosi' un peer irraggiungibile non blocca il round.
func New(selfAddr string, reg *registry.Registry, mem *membership.View, peersPerRound int, httpTimeout time.Duration) *Service {
	return &Service{
		SelfAddr:      selfAddr,
		Registry:      reg,
		Membership:    mem,
		Client:        &http.Client{Timeout: httpTimeout},
		PeersPerRound: peersPerRound,
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Start avvia in background il loop periodico di gossip e lo sweep
// periodico di failure detection, finche' ctx non viene cancellato.
func (s *Service) Start(ctx context.Context, roundInterval time.Duration) {
	go s.loop(ctx, roundInterval, s.doRound)
	go s.loop(ctx, roundInterval, s.doSweep)
}

func (s *Service) loop(ctx context.Context, interval time.Duration, step func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			step()
		}
	}
}

// doRound esegue un singolo round di gossip: selectPeer + selectToSend +
// scambio push-pull + selectToKeep/processData (ARCHITETTURA.md, sezione 6).
func (s *Service) doRound() {
	peers := s.Membership.AlivePeers()
	if len(peers) == 0 {
		return
	}

	n := s.PeersPerRound
	if n > len(peers) {
		n = len(peers)
	}
	s.rng.Shuffle(len(peers), func(i, j int) { peers[i], peers[j] = peers[j], peers[i] })

	for _, peer := range peers[:n] {
		if err := s.exchangeWith(peer); err != nil {
			log.Printf("gossip: scambio con %s fallito: %v", peer, err)
			continue
		}
		s.Membership.RecordContact(peer)
	}
}

func (s *Service) exchangeWith(peerAddr string) error {
	local := s.Registry.Snapshot()
	payload := exchangePayload{SenderAddr: s.SelfAddr, Entries: make([]wireEntry, len(local))}
	for i, e := range local {
		payload.Entries[i] = toWire(e)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("http://%s%s", peerAddr, ExchangePath)
	resp, err := s.Client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("peer ha risposto con status %d", resp.StatusCode)
	}

	var remote exchangePayload
	if err := json.NewDecoder(resp.Body).Decode(&remote); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	for _, w := range remote.Entries {
		s.Registry.Merge(fromWire(w))
	}
	return nil
}

// doSweep esegue la failure detection periodica (ARCHITETTURA.md, sezione
// 7): i peer che superano TimeoutDead vengono marcati DEAD e le loro entry
// nel registro vengono rimosse (tombstone), pronte a propagarsi al prossimo
// round di gossip.
func (s *Service) doSweep() {
	_, dead := s.Membership.Sweep()
	for _, addr := range dead {
		removed := s.Registry.RemoveEntriesOwnedBy(addr)
		if len(removed) > 0 {
			log.Printf("membership: nodo %s marcato DEAD, rimosse %d service entry", addr, len(removed))
		}
	}
}

// ExchangeHandler e' il lato server dello scambio anti-entropy: applica le
// entry ricevute dal peer chiamante (push), impara/riconferma il suo
// indirizzo in membership, e risponde con il proprio stato locale
// aggiornato (pull) — tutto in una singola interazione HTTP
// (ARCHITETTURA.md, sezione 6).
func (s *Service) ExchangeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var incoming exchangePayload
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	for _, wireE := range incoming.Entries {
		s.Registry.Merge(fromWire(wireE))
	}
	if incoming.SenderAddr != "" && incoming.SenderAddr != s.SelfAddr {
		s.Membership.RecordContact(incoming.SenderAddr)
	}

	local := s.Registry.Snapshot()
	response := exchangePayload{SenderAddr: s.SelfAddr, Entries: make([]wireEntry, len(local))}
	for i, e := range local {
		response.Entries[i] = toWire(e)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("gossip: errore scrivendo la risposta di exchange: %v", err)
	}
}
