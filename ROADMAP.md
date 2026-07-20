# Progetto B1 — Service Registry Distribuito
Scaletta delle fasi (deadline consegna: 18 settembre 2026)

## Fase 1 — Studio e progettazione architetturale
- Ripasso concetti: gossip protocol, failure detection (heartbeat/ping-ack), eventual consistency, version vector / timestamp + Last-Write-Wins
- Decisioni di design:
  - Come i nodi si scoprono all'avvio (seed nodes / bootstrap list da config)
  - Protocollo di gossip per propagare lo stato del registro tra nodi
  - Meccanismo di failure detection per rilevare nodi crashati/irraggiungibili
  - Formato dei dati: service ID, endpoint, metadata, timestamp/versione
  - API esposte: Register, Deregister, Discover/Lookup, (Heartbeat interno)
  - Comunicazione tra nodi: REST/HTTP o gRPC
- Output: schema architetturale (da riusare nella relazione)

## Fase 2 — Setup ambiente
- Installare Go, Docker Desktop, Git
- Creare repo Git (locale + GitHub)
- Struttura progetto Go (cmd/, internal/, pkg/, go.mod)
- File di configurazione (yaml/json/env) per i parametri: porta, intervallo di gossip, timeout, lista seed nodes — nessun valore hard-coded

## Fase 3 — Implementazione core del nodo
- Stato locale del registro (mappa servizi + metadati)
- API di registrazione/deregistrazione/discovery
- Loop di gossip periodico per scambio stato tra nodi
- Failure detection (ping/heartbeat + timeout) e marcatura nodi/servizi come stale

## Fase 4 — Consistenza e tolleranza ai guasti
- Meccanismo di riconciliazione stato (anti-entropy) per convergenza eventuale
- Gestione crash di un nodo: il resto del cluster continua a funzionare
- (Opzionale/bonus) recovery: un nodo che rientra dopo un crash si riallinea

## Fase 5 — Testing
- Unit test dei componenti principali (Go testing)
- Test di integrazione multi-nodo in locale
- Scenario: aggiunta/rimozione dinamica di servizi
- Scenario: simulazione crash di nodo (kill container) → verifica resilienza e consistenza
- (Opzionale) test di scalabilità al variare del numero di nodi/servizi

## Fase 6 — Containerizzazione e deploy
- Dockerfile per il nodo del registry
- docker-compose.yml per cluster multi-nodo
- Deploy e verifica su istanza EC2 (AWS Learner Lab)

## Fase 7 — Documentazione
- README: installazione, configurazione, esecuzione
- Commenti nel codice sorgente
- Relazione scientifica (max 5 pagine, template ACM o IEEE):
  architettura e scelte progettuali, implementazione, piattaforma/librerie usate,
  risultati dei test, limitazioni riscontrate

## Fase 8 — Rifinitura e consegna
- Review finale di codice e relazione
- Preparazione slide + demo live
- Invio email alla docente con link al repo/cloud storage (entro il 18/09/2026)
