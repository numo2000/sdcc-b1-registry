// Package membership mantiene la vista locale dei peer conosciuti
// (ALIVE / SUSPECTED / DEAD) e implementa la failure detection basata su
// heartbeat implicito nei round di gossip e timeout a due soglie.
//
// Vedi ARCHITETTURA.md, sezioni 4 e 7.
package membership

import (
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

// PeerInfo descrive un peer conosciuto e il suo stato di liveness osservato
// localmente.
type PeerInfo struct {
	Address       string
	State         State
	LastContactAt time.Time
}

// View e' la vista locale della membership, aggiornata dai round di gossip
// (ARCHITETTURA.md, sezione 6) e periodicamente ispezionata da Sweep per la
// failure detection (ARCHITETTURA.md, sezione 7).
//
// Semplificazione rispetto alla sezione 4 dell'architettura: in questa prima
// implementazione la vista e' puramente locale (basata sui contatti diretti
// del nodo), senza propagare via gossip lo stato di membership altrui. E'
// un punto di estensione naturale per la Fase 4 (tolleranza ai guasti).
type View struct {
	mu             sync.Mutex
	timeoutSuspect time.Duration
	timeoutDead    time.Duration
	peers          map[string]PeerInfo
	now            func() time.Time
}

// New crea una vista di membership vuota. timeoutSuspect e timeoutDead sono
// le soglie di ARCHITETTURA.md sezione 4 (timeout_suspect_ms/timeout_dead_ms
// in configs/config.example.yaml).
func New(timeoutSuspect, timeoutDead time.Duration) *View {
	return &View{
		timeoutSuspect: timeoutSuspect,
		timeoutDead:    timeoutDead,
		peers:          make(map[string]PeerInfo),
		now:            time.Now,
	}
}

// Seed registra gli indirizzi noti al bootstrap (seed list di configurazione,
// ARCHITETTURA.md sezione 3) come ALIVE, dando loro una finestra di timeout
// completa per confermarsi al primo round di gossip prima di degradare.
func (v *View) Seed(addresses []string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := v.now()
	for _, addr := range addresses {
		if _, ok := v.peers[addr]; !ok {
			v.peers[addr] = PeerInfo{Address: addr, State: StateAlive, LastContactAt: now}
		}
	}
}

// RecordContact registra un contatto riuscito con il peer (scambio di
// gossip completato) e lo marca ALIVE. Se il peer era DEAD, questo lo
// tratta come un nuovo join, coerentemente col modello fail-stop
// (ARCHITETTURA.md, sezione 4): non "risuscita" lo stato precedente, ne
// crea uno nuovo.
func (v *View) RecordContact(address string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.peers[address] = PeerInfo{Address: address, State: StateAlive, LastContactAt: v.now()}
}

// Sweep valuta il timeout di ogni peer conosciuto e ne aggiorna lo stato.
// Restituisce gli indirizzi che sono transitati a SUSPECTED o a DEAD in
// questa chiamata (ARCHITETTURA.md, sezione 7), cosi' il chiamante puo'
// reagire (es. rimuovere dal registry le entry possedute da un peer DEAD).
func (v *View) Sweep() (newlySuspected []string, newlyDead []string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := v.now()
	for addr, info := range v.peers {
		if info.State == StateDead {
			continue
		}
		elapsed := now.Sub(info.LastContactAt)
		switch {
		case elapsed >= v.timeoutDead:
			info.State = StateDead
			v.peers[addr] = info
			newlyDead = append(newlyDead, addr)
		case elapsed >= v.timeoutSuspect && info.State == StateAlive:
			info.State = StateSuspected
			v.peers[addr] = info
			newlySuspected = append(newlySuspected, addr)
		}
	}
	return newlySuspected, newlyDead
}

// AlivePeers restituisce gli indirizzi dei peer correntemente ALIVE o
// SUSPECTED, candidati per selectPeer nel round di gossip (ARCHITETTURA.md,
// sezione 6) — un peer SUSPECTED va comunque ricontattato: e' cosi' che si
// riconferma ALIVE prima di scadere come DEAD.
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

// Snapshot restituisce lo stato corrente di tutti i peer conosciuti.
func (v *View) Snapshot() []PeerInfo {
	v.mu.Lock()
	defer v.mu.Unlock()

	result := make([]PeerInfo, 0, len(v.peers))
	for _, info := range v.peers {
		result = append(result, info)
	}
	return result
}
