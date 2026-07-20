// Package registry mantiene lo stato locale del service registry: la mappa
// dei ServiceEntry con relativo versioning (Counter, OwnerNodeID) e la logica
// di riconciliazione last-write-wins.
//
// Vedi ARCHITETTURA.md, sezioni 5 e 9.
package registry

import "sync"

// Status indica se una ServiceEntry e' attiva o e' stata rimossa.
// Le entry rimosse non vengono cancellate subito dalla mappa: restano come
// "tombstone" versionato, cosi' la rimozione si propaga correttamente via
// gossip invece di essere ignorata da un nodo che possiede ancora la
// versione attiva precedente (vedi ARCHITETTURA.md, sezione 7).
type Status string

const (
	StatusActive  Status = "ACTIVE"
	StatusRemoved Status = "REMOVED"
)

// Version identifica la versione di una ServiceEntry con la coppia
// (Counter, OwnerNodeID), costruita come il clock scalare di Lamport con
// tie-break sull'ID del nodo (DS_Synchronization.pdf, slide 18-20;
// DS_MutualExclusion+Election.pdf, slide 6).
type Version struct {
	Counter     uint64
	OwnerNodeID string
}

// IsNewerThan implementa l'ordinamento totale usato per la riconciliazione
// last-write-wins (DS_Consistency.pdf, slide 17): a parita' di Counter,
// vince l'OwnerNodeID lessicograficamente maggiore.
func (v Version) IsNewerThan(other Version) bool {
	if v.Counter != other.Counter {
		return v.Counter > other.Counter
	}
	return v.OwnerNodeID > other.OwnerNodeID
}

// ServiceEntry e' una voce del registro distribuito.
type ServiceEntry struct {
	ServiceID string
	Endpoint  string
	Metadata  map[string]string
	Version   Version
	Status    Status
}

// Registry mantiene la copia locale, replicata, dello stato del service
// registry (ARCHITETTURA.md, sezione 3: overlay non strutturato + piena
// replica).
type Registry struct {
	mu       sync.RWMutex
	nodeID   string
	entries  map[string]ServiceEntry
}

// New crea un registry locale vuoto per il nodo con id nodeID.
func New(nodeID string) *Registry {
	return &Registry{
		nodeID:  nodeID,
		entries: make(map[string]ServiceEntry),
	}
}

// Register crea o aggiorna una entry con una nuova versione stampata dal
// nodo locale (Counter+1, nodeID) — ARCHITETTURA.md, sezione 8.
func (r *Registry) Register(serviceID, endpoint string, metadata map[string]string) ServiceEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	counter := uint64(1)
	if existing, ok := r.entries[serviceID]; ok {
		counter = existing.Version.Counter + 1
	}

	entry := ServiceEntry{
		ServiceID: serviceID,
		Endpoint:  endpoint,
		Metadata:  metadata,
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

// Discover restituisce la entry per serviceID, se presente e attiva.
func (r *Registry) Discover(serviceID string) (ServiceEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.entries[serviceID]
	if !ok || entry.Status != StatusActive {
		return ServiceEntry{}, false
	}
	return entry, true
}

// List restituisce tutte le entry attive del registro locale.
func (r *Registry) List() []ServiceEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ServiceEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		if entry.Status == StatusActive {
			result = append(result, entry)
		}
	}
	return result
}

// Snapshot restituisce tutte le entry (incluse quelle rimosse/tombstone),
// usato dal protocollo di gossip per lo scambio push-pull con i peer
// (ARCHITETTURA.md, sezione 6).
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
// versione piu' recente secondo last-write-wins (ARCHITETTURA.md, sezioni
// 6 e 9: selectToKeep del framework di gossip). Restituisce true se la
// entry incoming e' stata applicata (era piu' recente di quella locale).
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

// RemoveEntriesOwnedBy marca come rimosse tutte le entry attive di cui
// deadNodeID e' l'ultimo scrittore, in seguito al rilevamento del crash del
// nodo (ARCHITETTURA.md, sezione 7). La rimozione, come ogni altro update,
// si propaga poi via gossip.
func (r *Registry) RemoveEntriesOwnedBy(deadNodeID string) []ServiceEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	var removed []ServiceEntry
	for id, entry := range r.entries {
		if entry.Status == StatusActive && entry.Version.OwnerNodeID == deadNodeID {
			entry.Version = Version{Counter: entry.Version.Counter + 1, OwnerNodeID: r.nodeID}
			entry.Status = StatusRemoved
			r.entries[id] = entry
			removed = append(removed, entry)
		}
	}
	return removed
}
