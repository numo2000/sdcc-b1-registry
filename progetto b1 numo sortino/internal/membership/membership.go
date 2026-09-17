// Package membership mantiene la vista locale dei peer conosciuti
// (ALIVE / SUSPECTED / DEAD) e implementa la failure detection basata su
// heartbeat implicito nei round di gossip e timeout a due soglie.
package membership

import (
	"log"
	"sort"
	"sync"
	"time"
)

// State e' lo stato di liveness di un peer nella vista locale del nodo.
type State string

const (
	StateAlive     State = "ALIVE"
	StateSuspected State = "SUSPECTED"
	StateDead      State = "DEAD"
)

// PeerInfo descrive un peer conosciuto e il suo stato di liveness.
type PeerInfo struct {
	Address string
	State   State

	// Counter e OwnerNodeID identificano la versione di questa dichiarazione
	// di stato (clock scalare di Lamport, stessa tecnica di
	// registry.Version): ogni nuova osservazione su questo peer (contatto
	// diretto, o conclusione di Sweep) genera un Counter piu' alto,
	// attribuito a se stesso, cosi' i nodi confrontano in modo affidabile
	// quale opinione su un peer sia la piu' recente senza confrontare
	// orologi fisici diversi.
	Counter     uint64
	OwnerNodeID string

	// LastContactAt e' sempre scritto con l'orologio LOCALE di questo nodo,
	// mai importato da una dichiarazione remota via Merge - tiene la
	// failure detection locale indipendente dalla sincronizzazione degli
	// orologi tra macchine diverse.
	LastContactAt time.Time
}

// IsNewerThan implementa lo stesso ordinamento totale di
// registry.Version.IsNewerThan: a parita' di Counter, vince l'OwnerNodeID
// lessicograficamente maggiore.
func (p PeerInfo) IsNewerThan(other PeerInfo) bool {
	if p.Counter != other.Counter {
		return p.Counter > other.Counter
	}
	return p.OwnerNodeID > other.OwnerNodeID
}

// View e' la vista locale della membership, aggiornata dai round di gossip
// e periodicamente ispezionata da Sweep per la failure detection.
type View struct {
	mu sync.Mutex

	selfID string

	timeoutSuspect time.Duration
	timeoutDead    time.Duration

	peers map[string]PeerInfo
}

// New crea una vista di membership vuota per il nodo con indirizzo selfID.
// timeoutSuspect e timeoutDead sono le soglie di failure detection.
func New(selfID string, timeoutSuspect, timeoutDead time.Duration) *View {
	return &View{
		selfID:         selfID,
		timeoutSuspect: timeoutSuspect,
		timeoutDead:    timeoutDead,
		peers:          make(map[string]PeerInfo),
	}
}

// nextCounter calcola il prossimo valore di Counter per addr, ripartendo dal
// piu' alto gia' conosciuto (chiamare a mano, dentro un blocco gia' protetto
// da v.mu). Analogo al calcolo del counter in Registry.Register.
func (v *View) nextCounter(addr string) uint64 {
	if existing, ok := v.peers[addr]; ok {
		return existing.Counter + 1
	}
	return 1
}

// Seed registra gli indirizzi noti al bootstrap (seed list di configurazione)
// come ALIVE, dando loro una finestra di timeout completa per confermarsi al
// primo round di gossip prima di degradare.
func (v *View) Seed(addresses []string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	for _, addr := range addresses {
		if _, ok := v.peers[addr]; !ok {
			v.peers[addr] = PeerInfo{
				Address:       addr,
				State:         StateAlive,
				Counter:       v.nextCounter(addr),
				OwnerNodeID:   v.selfID,
				LastContactAt: now,
			}
		}
	}
}

// RecordContact registra un contatto riuscito con il peer (scambio di
// gossip completato) e lo marca ALIVE, con una nuova dichiarazione di
// versione attribuita a questo nodo. Se il peer era DEAD, questo lo tratta
// come un nuovo join, coerentemente col modello fail-stop: non "risuscita"
// lo stato precedente, ne crea uno nuovo.
func (v *View) RecordContact(address string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.peers[address] = PeerInfo{
		Address:       address,
		State:         StateAlive,
		Counter:       v.nextCounter(address),
		OwnerNodeID:   v.selfID,
		LastContactAt: time.Now(),
	}
}

// Sweep valuta il timeout di ogni peer conosciuto e ne aggiorna lo stato.
// Ogni transizione produce anche una nuova dichiarazione di versione, cosi'
// che la conclusione (anche pessimistica: SUSPECTED/DEAD) possa propagarsi
// via gossip tramite Merge, non solo le conferme positive. Restituisce gli
// indirizzi transitati a SUSPECTED o DEAD in questa chiamata - il crash di
// un nodo non causa la perdita delle sue service entry, che restano al
// sicuro sugli altri nodi grazie alla piena replica.
func (v *View) Sweep() (newlySuspected []string, newlyDead []string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	for addr, info := range v.peers {
		if info.State == StateDead {
			continue
		}
		elapsed := now.Sub(info.LastContactAt)
		switch {
		case elapsed >= v.timeoutDead:
			info.State = StateDead
			info.Counter = v.nextCounter(addr)
			info.OwnerNodeID = v.selfID
			v.peers[addr] = info
			newlyDead = append(newlyDead, addr)
		case elapsed >= v.timeoutSuspect && info.State == StateAlive:
			info.State = StateSuspected
			info.Counter = v.nextCounter(addr)
			info.OwnerNodeID = v.selfID
			v.peers[addr] = info
			newlySuspected = append(newlySuspected, addr)
		}
	}
	return newlySuspected, newlyDead
}

// Merge applica una dichiarazione di stato ricevuta da un peer via gossip,
// mantenendo quella con versione piu' recente secondo last-write-wins
// (stessa logica di Registry.Merge). Se la dichiarazione adottata e' ALIVE,
// resetta anche LastContactAt all'orologio locale di questo nodo, trattando
// la conferma come se fosse stata osservata direttamente; una dichiarazione
// SUSPECTED/DEAD non tocca LastContactAt. Restituisce true se applicata.
func (v *View) Merge(incoming PeerInfo) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	if incoming.Address == "" || incoming.Address == v.selfID {
		return false
	}

	existing, ok := v.peers[incoming.Address]
	if ok && !incoming.IsNewerThan(existing) {
		return false
	}

	if incoming.State == StateAlive {
		incoming.LastContactAt = time.Now()
		log.Printf("membership: %s rinfresca il timer di %s via claim ricevuto (counter=%d, owner=%s)", v.selfID, incoming.Address, incoming.Counter, incoming.OwnerNodeID)
	} else {
		incoming.LastContactAt = existing.LastContactAt
	}
	v.peers[incoming.Address] = incoming
	return true
}

// Snapshot restituisce il PeerInfo completo di ogni peer conosciuto - usato
// dal protocollo di gossip per propagare anche le dichiarazioni di
// membership, non solo i ServiceEntry.
func (v *View) Snapshot() []PeerInfo {
	v.mu.Lock()
	defer v.mu.Unlock()

	result := make([]PeerInfo, 0, len(v.peers))
	for _, info := range v.peers {
		result = append(result, info)
	}
	return result
}

// AlivePeers restituisce gli indirizzi dei peer correntemente ALIVE o
// SUSPECTED, candidati per selectPeer nel round di gossip — un peer
// SUSPECTED va comunque ricontattato: e' cosi' che si riconferma ALIVE
// prima di scadere come DEAD.
func (v *View) AlivePeers() []string {
	v.mu.Lock()
	defer v.mu.Unlock()

	var result []string
	for addr, info := range v.peers {
		if info.State != StateDead {
			result = append(result, addr)
		}
	}
	return result
}

// SelectLeastContacted restituisce fino a n indirizzi tra i peer ALIVE o
// SUSPECTED, scegliendo quelli con il contatto meno recente. E'
// un'alternativa deterministica al selectPeer casuale: garantisce che
// nessun peer resti trascurato a lungo per pura sfortuna nella selezione.
//
// Se non ci sono candidati ALIVE/SUSPECTED (fallback == true), ripesca
// TUTTI i DEAD ignorando il limite n, invece di limitarsi ai primi n:
// altrimenti un collasso correlato (tutti i nodi marcano tutti gli altri
// DEAD nello stesso momento, es. dopo una pausa dell'intera VM) lascerebbe
// il cluster bloccato per sempre, e limitarsi a un solo DEAD per round
// frammenterebbe il cluster in isole invece di ricomporre la mesh piena.
// Il valore fallback dice al chiamante (gossip.doRound) quale dei due casi
// gestire.
func (v *View) SelectLeastContacted(n int) (addrs []string, fallback bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	candidates := make([]PeerInfo, 0, len(v.peers))
	for _, info := range v.peers {
		if info.State != StateDead {
			candidates = append(candidates, info)
		}
	}

	fallback = len(candidates) == 0
	if fallback {
		for _, info := range v.peers {
			candidates = append(candidates, info)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].LastContactAt.Before(candidates[j].LastContactAt)
	})

	limit := n
	if fallback {
		limit = len(candidates)
	}
	if limit > len(candidates) {
		limit = len(candidates)
	}
	addrs = make([]string, limit)
	for i := 0; i < limit; i++ {
		addrs[i] = candidates[i].Address
	}
	return addrs, fallback
}

// SelectLeastContactedDead restituisce fino a n indirizzi tra i soli peer
// DEAD, scegliendo quelli con il contatto meno recente - stessa logica di
// SelectLeastContacted ma ristretta ai DEAD.
//
// Serve al controllo periodico anti-partizione in gossip.doRound, per il
// caso che il fallback di SelectLeastContacted non copre: una partizione di
// rete in cui ogni lato ha ancora peer ALIVE al proprio interno (quindi il
// fallback non scatta mai), ma nessuno dei due lati avrebbe piu' motivo di
// ricontattare l'altro una volta marcato DEAD - senza questo controllo
// periodico, indipendente dallo stato del pool ALIVE, una partizione sanata
// a livello di rete non guarirebbe mai a livello applicativo.
func (v *View) SelectLeastContactedDead(n int) []string {
	v.mu.Lock()
	defer v.mu.Unlock()

	candidates := make([]PeerInfo, 0, len(v.peers))
	for _, info := range v.peers {
		if info.State == StateDead {
			candidates = append(candidates, info)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].LastContactAt.Before(candidates[j].LastContactAt)
	})

	if n > len(candidates) {
		n = len(candidates)
	}
	result := make([]string, n)
	for i := 0; i < n; i++ {
		result[i] = candidates[i].Address
	}
	return result
}
