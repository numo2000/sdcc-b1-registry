# SDCC B1 — Service Registry Distribuito

Service registry decentralizzato e fault-tolerant, progetto B1 del corso Sistemi Distribuiti e Cloud Computing (Tor Vergata, A.A. 2025/26).

## Documentazione di progetto
- [ROADMAP.md](ROADMAP.md) — fasi di sviluppo
- [TEORIA_RIPASSO.md](TEORIA_RIPASSO.md) — ripasso teorico basato sulle slide del corso
- [ARCHITETTURA.md](ARCHITETTURA.md) — scelte architetturali e design

## Struttura del progetto
```
cmd/registry-node/    entrypoint del binario del nodo
internal/registry/    stato locale del registro e versioning
internal/gossip/      protocollo di gossip (anti-entropy push-pull)
internal/membership/  vista dei peer e failure detection
internal/api/         API client-facing (Register/Deregister/Discover)
internal/config/      caricamento configurazione
configs/               file di configurazione di esempio
```

## Stato del progetto
In sviluppo — vedi [ROADMAP.md](ROADMAP.md) per le fasi completate.

## Installazione ed esecuzione
_TODO: da completare nella Fase 7 (documentazione finale)._
