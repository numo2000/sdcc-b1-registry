# SDCC B1 — Service Registry Distribuito

Service registry decentralizzato e fault-tolerant, progetto B1 del corso Sistemi Distribuiti e Cloud Computing (Tor Vergata, A.A. 2025/26).

## Documentazione di progetto
- [ARCHITETTURA.md](ARCHITETTURA.md) — scelte architetturali e design

## Struttura del progetto
```
cmd/registry-node/    entrypoint del binario del nodo
internal/registry/    stato locale del registro e versioning
internal/gossip/      protocollo di gossip su gRPC (anti-entropy push-pull)
internal/gossippb/    codice generato da proto/gossip.proto (NON modificare a mano)
internal/membership/  vista dei peer e failure detection
internal/api/         API client-facing HTTP+JSON (Register/Deregister/Discover)
internal/config/      caricamento configurazione
configs/               file di configurazione di esempio
proto/                 definizione del servizio gRPC di gossip (gossip.proto)
```

## Installazione ed esecuzione

### Prerequisiti
- Docker Desktop (o Docker Engine + plugin Compose) in esecuzione
- `curl` per interagire con l'API (già presente su Windows, macOS, Linux)
- Una shell compatibile bash: su Linux/macOS il terminale predefinito va bene; su Windows serve **Git Bash** (incluso con [Git for Windows](https://git-scm.com/download/win)) — i comandi qui sotto non sono scritti per PowerShell o cmd.exe

### 0. Aprire un terminale nella cartella del progetto
Tutti i comandi di questa guida vanno lanciati dalla cartella che contiene questo `README.md` (quella con `docker-compose.yml`, `Dockerfile`, ecc.). Se il terminale si apre altrove, spostati lì prima con `cd`, ad esempio:
```bash
cd percorso/dove/hai/estratto/il/progetto
```
Puoi verificare di essere nella cartella giusta con `ls`: dovresti vedere, tra gli altri, i file `Dockerfile` e `docker-compose.yml`.

### 1. Generare la configurazione del cluster
Il numero di nodi è parametrico. Per un cluster di prova locale, 5 nodi bastano ed è veloce da avviare:
```bash
bash generate-configs.sh 5
bash generate-compose.sh 5
```
Questo rigenera `configs/node1.yaml`...`configs/node5.yaml` e `docker-compose.yml`. Per un numero diverso di nodi, basta cambiare l'argomento (es. `20`).

### 2. Avviare il cluster
Insieme al cluster avviamo anche `demo-clients`, un piccolo container che apre 10 porte TCP in ascolto reale (`9001`...`9010`) apposta per fare da "endpoint finto ma raggiungibile" per i servizi che registreremo nei passaggi seguenti — non è un nodo del registro, è solo un bersaglio comodo su cui puntare:
```bash
docker compose -f docker-compose.yml -f docker-compose.demo-clients.yml up -d --build
```
Ogni nodo è un container separato (`registry-node-1`...`registry-node-N`), sulla stessa rete Docker, con l'API HTTP esposta su `localhost:808N` (es. `registry-node-1` → `localhost:8081`).

Verifica che tutti i nodi (e `demo-clients`) siano su:
```bash
docker ps --format '{{.Names}}: {{.Status}}' | grep -E 'registry-node|demo-clients'
```

### 3. Registrare un servizio e verificarne la propagazione
```bash
curl -s -X POST http://localhost:8081/services \
  -H "Content-Type: application/json" \
  -d '{"service_id":"demo-svc","endpoint":"demo-clients:9001"}'
```
L'`endpoint` deve essere raggiungibile *dalla rete Docker interna* (non dal host) perché il controllo di raggiungibilità periodico lo contatti con successo: per questo puntiamo a `demo-clients`, non a un altro nodo del registro (il registro è protocol-agnostic, non gli importa cosa c'è dietro — potrebbe essere qualsiasi servizio vero).

Dopo un paio di secondi, verifica che il servizio sia visibile **da un nodo diverso da quello su cui hai scritto** (qui è la prova che il gossip ha propagato la scrittura):
```bash
curl -s http://localhost:8083/services/demo-svc
```

### 4. Rimuovere un servizio e verificarne la propagazione
```bash
curl -s -X DELETE http://localhost:8081/services/demo-svc
sleep 2
curl -s http://localhost:8083/services/demo-svc   # atteso: "service not found"
```

### 5. Simulare il crash di un nodo e verificare che il dato sopravviva
```bash
curl -s -X POST http://localhost:8082/services \
  -H "Content-Type: application/json" \
  -d '{"service_id":"crash-demo","endpoint":"demo-clients:9002"}'
docker kill registry-node-2
sleep 3
curl -s http://localhost:8084/services/crash-demo   # un nodo mai contattato direttamente: il dato c'e' comunque
```

### 6. Far ripartire il nodo e osservare il rejoin
```bash
docker start registry-node-2
sleep 8
curl -s http://localhost:8082/services/crash-demo   # registry-node-2 riparte vuoto e recupera lo stato via pull
```

### 7. Fermare il cluster
```bash
docker compose -f docker-compose.yml -f docker-compose.demo-clients.yml down
```
