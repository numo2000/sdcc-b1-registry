// Package registry mantiene lo stato locale del service registry: la mappa
// dei ServiceEntry con relativo versioning (Counter, OwnerNodeID) e la logica
// di riconciliazione last-write-wins.
package registry

import "sync"

// Status indica lo stato di una ServiceEntry. ACTIVE e SUSPECTED sono
// entrambe entry valide restituite da Discover/List (SUSPECTED segnala solo
// che l'ultimo controllo di raggiungibilita' sull'endpoint e' fallito, non
// che il servizio sia dato per morto - vedi Registry.UpdateHealth). Le entry
// rimosse restano come tombstone versionato invece di essere cancellate
// subito, cosi' la rimozione si propaga correttamente via gossip anche a un
// nodo che possiede ancora la versione attiva precedente.
type Status string

const (
	StatusActive    Status = "ACTIVE"
	StatusSuspected Status = "SUSPECTED"
	StatusRemoved   Status = "REMOVED"
)

// Version identifica la versione di una ServiceEntry con la coppia
// (Counter, OwnerNodeID): un clock scalare di Lamport con tie-break
// sull'ID del nodo.
type Version struct {
	Counter     uint64
	OwnerNodeID string
}

// IsNewerThan implementa l'ordinamento totale usato per la riconciliazione
// last-write-wins: a parita' di Counter, vince l'OwnerNodeID
// lessicograficamente maggiore.
func (v Version) IsNewerThan(other Version) bool {
	if v.Counter != other.Counter {
		return v.Counter > other.Counter
	}
	return v.OwnerNodeID > other.OwnerNodeID
}

// ServiceEntry e' una voce del registro distribuito, sia tenuta nella mappa
// interna di Registry sia serializzata via gRPC per lo scambio con i peer
// (toWire/fromWire in gossip.go).
type ServiceEntry struct {
	ServiceID string
	Endpoint  string
	Version   Version
	Status    Status
}

// Registry mantiene la copia locale, replicata, dello stato del service
// registry. mu protegge entries dall'accesso concorrente delle goroutine di
// questo stesso processo (handler HTTP, round di gossip, sweep) - non
// sincronizza nulla tra nodi diversi: quella e' responsabilita' del
// protocollo di gossip e del versioning.
type Registry struct {
	mu      sync.RWMutex
	nodeID  string
	entries map[string]ServiceEntry
}

// New crea un registry locale vuoto per il nodo con id nodeID.
func New(nodeID string) *Registry {
	return &Registry{
		nodeID:  nodeID,
		entries: make(map[string]ServiceEntry),
	}
}

// Register crea o aggiorna una entry con una nuova versione stampata dal
// nodo locale (Counter+1, nodeID).
func (r *Registry) Register(serviceID, endpoint string) ServiceEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	counter := uint64(1)
	if existing, ok := r.entries[serviceID]; ok {
		counter = existing.Version.Counter + 1
	}

	entry := ServiceEntry{
		ServiceID: serviceID,
		Endpoint:  endpoint,
		Version:   Version{Counter: counter, OwnerNodeID: r.nodeID},
		Status:    StatusActive,
	}
	r.entries[serviceID] = entry
	return entry
}

// Deregister marca una entry come rimossa con una nuova versione (tombstone).
// Se la entry non esiste, non fa nulla: non ha senso creare un tombstone per
// un servizio mai registrato.
func (r *Registry) Deregister(serviceID string) (ServiceEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.entries[serviceID]
	if !ok {
		return ServiceEntry{}, false
	}

	entry := existing
	entry.Version = Version{Counter: existing.Version.Counter + 1, OwnerNodeID: r.nodeID}
	entry.Status = StatusRemoved
	r.entries[serviceID] = entry
	return entry, true
}

// UpdateHealth aggiorna lo Status di un'entry ACTIVE/SUSPECTED in base
// all'esito di un controllo di raggiungibilita' sul suo endpoint (vedi
// gossip.Service.doHealthCheck), con una nuova versione attribuita a questo
// nodo - stessa tecnica di Register/Deregister. Non tocca le entry REMOVED
// (non ha senso "resuscitare" un tombstone con un controllo) ne' quelle
// gia' nello stato risultante (nessun cambiamento reale, nessuna versione
// sprecata). Restituisce true se lo stato e' effettivamente cambiato.
func (r *Registry) UpdateHealth(serviceID string, reachable bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.entries[serviceID]
	if !ok || existing.Status == StatusRemoved {
		return false
	}

	newStatus := StatusSuspected
	if reachable {
		newStatus = StatusActive
	}
	if existing.Status == newStatus {
		return false
	}

	existing.Status = newStatus
	existing.Version = Version{Counter: existing.Version.Counter + 1, OwnerNodeID: r.nodeID}
	r.entries[serviceID] = existing
	return true
}

// Discover restituisce la entry per serviceID, se presente e non rimossa
// (ACTIVE o SUSPECTED - il chiamante decide come trattare un endpoint
// SUSPECTED, es. riprovare lato client).
func (r *Registry) Discover(serviceID string) (ServiceEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.entries[serviceID]
	if !ok || entry.Status == StatusRemoved {
		return ServiceEntry{}, false
	}
	return entry, true
}

// List restituisce tutte le entry non rimosse del registro locale (ACTIVE o
// SUSPECTED), in ordine non garantito (iterazione su mappa).
func (r *Registry) List() []ServiceEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ServiceEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		if entry.Status != StatusRemoved {
			result = append(result, entry)
		}
	}
	return result
}

// Snapshot restituisce tutte le entry, incluse quelle rimosse/tombstone:
// usato dal protocollo di gossip per lo scambio push-pull con i peer, a cui
// servono anche i tombstone perche' la rimozione si propaghi ai peer che
// hanno ancora la versione attiva vecchia.
func (r *Registry) Snapshot() []ServiceEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ServiceEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		result = append(result, entry)
	}
	return result
}

// Merge applica una entry ricevuta da un peer, mantenendo quella con
// versione piu' recente secondo last-write-wins. Restituisce true se la
// entry incoming e' stata applicata.
func (r *Registry) Merge(incoming ServiceEntry) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.entries[incoming.ServiceID]
	if !ok || incoming.Version.IsNewerThan(existing.Version) {
		r.entries[incoming.ServiceID] = incoming
		return true
	}
	return false
}
