// Package gossip implementa il protocollo di anti-entropy push-pull
// (selectPeer, selectToSend, scambio, selectToKeep, processData) su gRPC
// per la propagazione dello stato del registry tra i nodi, e la failure
// detection basata su timeout sui contatti diretti.
package gossip

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"sdcc-b1-registry/internal/gossippb"
	"sdcc-b1-registry/internal/membership"
	"sdcc-b1-registry/internal/registry"
)

// toWire/fromWire convertono tra il formato interno (registry.ServiceEntry)
// e il formato wire generato dal .proto (gossippb.ServiceEntry), l'unico
// che gRPC sa serializzare per la rete.
func toWire(e registry.ServiceEntry) *gossippb.ServiceEntry {
	return &gossippb.ServiceEntry{
		ServiceId:   e.ServiceID,
		Endpoint:    e.Endpoint,
		Counter:     e.Version.Counter,
		OwnerNodeId: e.Version.OwnerNodeID,
		Status:      string(e.Status),
	}
}

// fromWire fa il percorso inverso di toWire.
func fromWire(w *gossippb.ServiceEntry) registry.ServiceEntry {
	return registry.ServiceEntry{
		ServiceID: w.ServiceId,
		Endpoint:  w.Endpoint,
		Version:   registry.Version{Counter: w.Counter, OwnerNodeID: w.OwnerNodeId},
		Status:    registry.Status(w.Status),
	}
}

// toWirePeer/fromWirePeer sono l'equivalente di toWire/fromWire per le
// dichiarazioni di membership (membership.PeerInfo <-> gossippb.PeerClaim).
func toWirePeer(p membership.PeerInfo) *gossippb.PeerClaim {
	return &gossippb.PeerClaim{
		Address:     p.Address,
		Counter:     p.Counter,
		OwnerNodeId: p.OwnerNodeID,
		State:       string(p.State),
	}
}

func fromWirePeer(w *gossippb.PeerClaim) membership.PeerInfo {
	return membership.PeerInfo{
		Address:     w.Address,
		Counter:     w.Counter,
		OwnerNodeID: w.OwnerNodeId,
		State:       membership.State(w.State),
	}
}

// Service coordina il protocollo di gossip anti-entropy push-pull su gRPC e
// la failure detection basata su timeout per un nodo del registry.
// Implementa anche gossippb.GossipServer: e' sia client che server gRPC,
// coerentemente col ruolo di "servent" di un nodo P2P.
type Service struct {
	// Embedding (campo senza nome, solo tipo): Service eredita i metodi
	// placeholder di UnimplementedGossipServer, richiesto per soddisfare
	// gossippb.GossipServer (che include un metodo non esportato
	// impossibile da implementare da fuori il package). Il vero Exchange
	// dichiarato piu' sotto prende il sopravvento su quello ereditato.
	gossippb.UnimplementedGossipServer

	SelfAddr   string
	Registry   *registry.Registry
	Membership *membership.View

	PeersPerRound int
	DialTimeout   time.Duration

	// DeadCheckEveryNRounds, se > 0, contatta anche il DEAD meno recente
	// ogni N round indipendentemente da PeersPerRound e dal pool ALIVE -
	// vedi doRound per il perche' (guarigione di una partizione di rete).
	// 0 disabilita il controllo.
	DeadCheckEveryNRounds int

	// roundCount conta i round eseguiti, per il controllo periodico sopra.
	// Letto/scritto solo da doRound, sempre nella stessa goroutine (vedi
	// Start/loop) - nessuna sincronizzazione necessaria.
	roundCount int

	// HealthCheckInterval e CheckTimeout controllano il controllo periodico
	// di raggiungibilita' sugli endpoint dei servizi registrati (vedi
	// doHealthCheck). HealthCheckInterval <= 0 disabilita il controllo.
	HealthCheckInterval time.Duration
	CheckTimeout        time.Duration
}

// New crea un Service di gossip. dialTimeout limita la durata di un singolo
// scambio con un peer, cosi' un peer irraggiungibile non blocca il round.
func New(selfAddr string, reg *registry.Registry, mem *membership.View, peersPerRound int, dialTimeout time.Duration, deadCheckEveryNRounds int, healthCheckInterval time.Duration, checkTimeout time.Duration) *Service {
	return &Service{
		SelfAddr:              selfAddr,
		Registry:              reg,
		Membership:            mem,
		PeersPerRound:         peersPerRound,
		DialTimeout:           dialTimeout,
		DeadCheckEveryNRounds: deadCheckEveryNRounds,
		HealthCheckInterval:   healthCheckInterval,
		CheckTimeout:          checkTimeout,
	}
}

// Start avvia in background il loop periodico di gossip e lo sweep di
// failure detection. Girano per tutta la vita del processo: il nodo non ha
// spegnimento ordinato, viene terminato direttamente dal SO, coerente col
// modello fail-stop.
func (s *Service) Start(roundInterval time.Duration) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds) // precisione ai microsecondi, per distinguere contatti ravvicinati nei log

	// round_interval_ms e' leggermente sfasato per nodo (vedi
	// configs/nodeN.yaml) per ridurre la probabilita' che nodi diversi
	// contattino lo stesso peer nello stesso istante.
	go s.loop(roundInterval, s.doRound)
	go s.loop(roundInterval, s.doSweep)

	// Ciclo separato per il controllo di raggiungibilita' dei servizi: gira
	// sul proprio intervallo (in genere piu' lungo del round di gossip),
	// indipendente apposta perche' non deve rallentare ne' il gossip ne'
	// il percorso di lettura (Discover resta una lookup locale).
	if s.HealthCheckInterval > 0 {
		go s.loop(s.HealthCheckInterval, s.doHealthCheck)
	}
}

// loop chiama step() ogni interval, per tutta la vita del processo. Ogni
// nodo ha il proprio ticker locale, indipendente e non sincronizzato con
// quello degli altri nodi.
func (s *Service) loop(interval time.Duration, step func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		step()
	}
}

// doRound esegue un singolo round di gossip: selectPeer + selectToSend +
// scambio push-pull + selectToKeep/processData. selectPeer usa la strategia
// "least-recently-contacted" (Membership.SelectLeastContacted), alternativa
// alla selezione uniforme casuale.
//
// Con peers_per_round > 1, ogni peer viene scelto ricalcolando
// SelectLeastContacted(1) subito prima di contattarlo, non tutti insieme a
// inizio round: cosi' un peer gia' rinfrescato indirettamente (via Merge)
// dal primo scambio del round non viene ricontattato inutilmente nello
// stesso round.
//
// Il fallback di SelectLeastContacted (pool ALIVE/SUSPECTED vuoto) va invece
// gestito contattando TUTTI i DEAD restituiti in un colpo solo, non un peer
// alla volta: altrimenti dopo il primo contatto riuscito il pool non
// sarebbe piu' vuoto e si fermerebbe li', frammentando il cluster in isole
// invece di ricomporre la mesh piena.
//
// Il fallback copre solo il collasso globale (pool ALIVE vuoto su questo
// nodo), non una partizione di rete: ogni lato di una partizione ha ancora
// peer ALIVE al proprio interno, quindi il fallback non scatta mai e i nodi
// dell'altro lato restano DEAD anche dopo che la rete torna disponibile.
// Per questo, ogni DeadCheckEveryNRounds round viene comunque contattato
// anche il DEAD meno recente (vedi maybeCheckDead) - basta a far
// riconvergere una partizione sanata, senza sprecare ogni round in contatti
// falliti verso nodi probabilmente morti per davvero.
func (s *Service) doRound() {
	s.roundCount++

	firstBatch, fallback := s.Membership.SelectLeastContacted(1)
	if len(firstBatch) > 0 {
		if fallback {
			for _, peer := range firstBatch {
				s.contactPeer(peer)
			}
			s.maybeCheckDead()
			return
		}

		s.contactPeer(firstBatch[0])
		for i := 1; i < s.PeersPerRound; i++ {
			peers, _ := s.Membership.SelectLeastContacted(1)
			if len(peers) == 0 {
				break
			}
			s.contactPeer(peers[0])
		}
	}

	s.maybeCheckDead()
}

// maybeCheckDead contatta il DEAD meno recentemente contattato ogni
// DeadCheckEveryNRounds round (0 disabilita), indipendentemente da quanti
// peer ALIVE esistano - vedi il commento su doRound sopra per il perche'
// (guarigione di una partizione di rete).
func (s *Service) maybeCheckDead() {
	if s.DeadCheckEveryNRounds <= 0 || s.roundCount%s.DeadCheckEveryNRounds != 0 {
		return
	}
	dead := s.Membership.SelectLeastContactedDead(1)
	if len(dead) == 0 {
		return
	}
	s.contactPeer(dead[0])
}

// contactPeer esegue un singolo scambio anti-entropy con peer e ne
// registra l'esito in membership (RecordContact solo se riuscito).
func (s *Service) contactPeer(peer string) {
	if err := s.exchangeWith(peer); err != nil {
		log.Printf("gossip: scambio con %s fallito: %v", peer, err)
		return
	}
	s.Membership.RecordContact(peer)
	log.Printf("gossip: scambio con %s riuscito", peer) // diagnostico, per osservare la rotazione dei peer
}

// exchangeWith apre una connessione gRPC verso peerAddr e invoca il metodo
// remoto Exchange: e' il lato client dello scambio anti-entropy.
func (s *Service) exchangeWith(peerAddr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.DialTimeout)
	defer cancel()

	conn, err := grpc.NewClient(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	client := gossippb.NewGossipClient(conn)

	local := s.Registry.Snapshot()
	peers := s.Membership.Snapshot()
	req := &gossippb.ExchangeRequest{
		SenderAddr: s.SelfAddr,
		Entries:    make([]*gossippb.ServiceEntry, len(local)),
		PeerClaims: make([]*gossippb.PeerClaim, len(peers)),
	}
	for i, e := range local {
		req.Entries[i] = toWire(e)
	}
	for i, p := range peers {
		req.PeerClaims[i] = toWirePeer(p)
	}

	resp, err := client.Exchange(ctx, req)
	if err != nil {
		return fmt.Errorf("exchange: %w", err)
	}
	for _, w := range resp.Entries {
		s.Registry.Merge(fromWire(w))
	}
	for _, pc := range resp.PeerClaims {
		s.Membership.Merge(fromWirePeer(pc))
	}
	return nil
}

// doSweep esegue la failure detection periodica: i peer oltre TimeoutDead
// vengono marcati DEAD. I servizi che avevano registrato tramite quel nodo
// restano ACTIVE nel registro grazie alla piena replica - il crash di un
// nodo non causa perdita di dati.
func (s *Service) doSweep() {
	_, dead := s.Membership.Sweep()
	for _, addr := range dead {
		log.Printf("membership: nodo %s marcato DEAD", addr)
	}
}

// doHealthCheck controlla la raggiungibilita' (TCP dial con timeout breve)
// di ogni servizio non rimosso conosciuto localmente, e ne aggiorna lo
// Status (ACTIVE/SUSPECTED) di conseguenza tramite Registry.UpdateHealth -
// stessa idea della failure detection sui nodi (doSweep), applicata pero'
// ai servizi registrati invece che ai peer. Gira su un ciclo separato dal
// round di gossip: Discover non contatta mai il servizio a runtime, legge
// solo lo Status gia' calcolato qui.
func (s *Service) doHealthCheck() {
	for _, entry := range s.Registry.Snapshot() {
		if entry.Status == registry.StatusRemoved {
			continue
		}
		reachable := s.checkReachable(entry.Endpoint)
		if s.Registry.UpdateHealth(entry.ServiceID, reachable) {
			newStatus := registry.StatusSuspected
			if reachable {
				newStatus = registry.StatusActive
			}
			log.Printf("registry: servizio %s (%s) segnato %s dopo controllo di raggiungibilita'", entry.ServiceID, entry.Endpoint, newStatus)
		}
	}
}

// checkReachable apre e chiude subito una connessione TCP verso endpoint:
// e' l'unico controllo protocol-agnostic possibile, dato che il registry
// non sa nulla del protocollo applicativo del servizio registrato (potrebbe
// essere qualunque cosa dietro quell'indirizzo).
func (s *Service) checkReachable(endpoint string) bool {
	conn, err := net.DialTimeout("tcp", endpoint, s.CheckTimeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Exchange e' il lato server dello scambio anti-entropy (implementa
// gossippb.GossipServer): applica le entry ricevute dal peer chiamante
// (push), impara/riconferma il suo indirizzo in membership, e risponde con
// il proprio stato locale aggiornato (pull) — tutto in una singola chiamata
// gRPC.
func (s *Service) Exchange(ctx context.Context, req *gossippb.ExchangeRequest) (*gossippb.ExchangeResponse, error) {
	for _, w := range req.Entries {
		s.Registry.Merge(fromWire(w))
	}
	if req.SenderAddr != "" && req.SenderAddr != s.SelfAddr {
		s.Membership.RecordContact(req.SenderAddr)
	}
	for _, pc := range req.PeerClaims {
		s.Membership.Merge(fromWirePeer(pc))
	}

	local := s.Registry.Snapshot()
	peers := s.Membership.Snapshot()
	resp := &gossippb.ExchangeResponse{
		Entries:    make([]*gossippb.ServiceEntry, len(local)),
		PeerClaims: make([]*gossippb.PeerClaim, len(peers)),
	}
	for i, e := range local {
		resp.Entries[i] = toWire(e)
	}
	for i, p := range peers {
		resp.PeerClaims[i] = toWirePeer(p)
	}
	return resp, nil
}
