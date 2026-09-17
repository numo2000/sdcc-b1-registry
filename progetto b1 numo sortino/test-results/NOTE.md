# Note metodologiche sui test (tolte dai CSV per non romperne la struttura tabellare)

Suite finale: 2 esperimenti su cluster EC2 (Docker Compose), scala indicata caso per caso.

## test1_addremove_fotografia_16scan.csv
Registrazione/rimozione dinamica dei servizi con fotografia dell'evoluzione nel tempo, su un
cluster isolato a 10 nodi: 16 scansioni di List() su tutti i nodi, a intervallo fisso di 0.75s
(scanner in background), mentre in parallelo si susseguono azioni di registrazione/rimozione.
Sequenza: fase A, registrazione di 8 servizi (S1..S8, uno per nodo, staggered, t=0-2s); fase B
(prima meta'), rimozione di S1 e S2 (t~3s), poi aggiunta di S9 e S10 (t~6s); fase C (seconda
meta'), rimozione di S3, S4, S5 (t~9s), fino alla stabilizzazione finale a 5 servizi attivi.
Le scansioni sono a intervallo fisso indipendente dalle azioni: questo permette di catturare
stati intermedi (disomogenei tra nodi) sia dopo la prima rimozione (scansione 5, valori 7-8)
sia dopo l'aggiunta (scansione 8, valori 6-7) sia dopo la seconda rimozione (scansione 12,
valori 5-8), non solo lo stato finale convergente.

## test2_crash_push_pull_80trial.csv / test2_crash_riepilogo_per_fascia.csv
Ogni nodo crasha a una frazione del proprio round_interval_ms individuale (formula a sfasamento
modulo), per un totale di 80 trial (20 nodi x 4 frazioni: 1/12, 1/10, 1/8, 1/4 - fasce strette
per isolare la "zona di rischio" dove il crash avviene molto presto nel round del nodo). Per
ogni trial si registra un servizio sul nodo, si attende il delay calcolato, si uccide il nodo
(docker kill), e si classifica - tramite finestra temporale sui timestamp di log (log.
LstdFlags|Lmicroseconds) - se il servizio e' stato "salvato" dal PUSH del nodo stesso (il suo
round di gossip uscente prima di morire) o dal PULL di un altro nodo (che lo ha contattato
indipendentemente prima che morisse). Obiettivo: verificare che un nodo morto molto presto nel
proprio round non debba necessariamente propagare il proprio servizio, a meno che un altro nodo
non lo contatti nel frattempo (pull).
Risultato: sopravvivenza crescente con la frazione - 35% a 1/12 di round, 50% a 1/10, 70% a
1/8, 95% a 1/4. Dato piu' significativo: in NESSUNA fascia si osserva un salvataggio dovuto al
solo push (rescued_by_push_only=0 su tutte e quattro le righe) - ogni sopravvivenza rilevata
e' dovuta al pull (da solo o assieme al push), confermando che a tempi cosi' brevi il nodo
morente non fa quasi mai in tempo a completare un proprio round di gossip uscente prima di
morire: e' quasi sempre un altro nodo a contattarlo per primo (pull) a salvare il dato.

Nota metodologica: il delay (frazione x round_interval_ms) e' misurato dal momento della
registrazione del servizio, non dall'istante in cui il ticker di gossip del nodo (time.
NewTicker, gossip.go) e' partito - i due eventi non coincidono, dato che nel test il nodo
viene riavviato prima di ogni trial e la registrazione avviene con un ritardo fisso (~0.8s)
rispetto al riavvio, indipendente dalla fase del ticker. Per questo alcuni trial a 1/8 di
round mostrano comunque un push riuscito: non e' un'anomalia del sistema, ma un limite del
sincronismo tra script di test e ciclo di gossip del nodo.

## Rieseguire i due scenari (versione demo)
`scripts/test1_addremove_scan.sh` e `scripts/test2_crash_pushpull.sh` sono versioni piu'
leggere dei due esperimenti sopra (meno prove ripetute, pensate per una demo dal vivo invece
che per produrre dati), ma con la stessa identica metodologia: scansioni periodiche via API
per il primo, analisi dei timestamp nei log del container per il secondo. Presuppongono il
cluster gia' avviato (`docker compose up -d`, vedi README) su localhost. Si lanciano dalla
cartella del progetto:
```bash
bash scripts/test1_addremove_scan.sh 5
bash scripts/test2_crash_pushpull.sh 5 0.25
```
Il primo argomento (5) e' il numero di nodi del cluster; per test2 il secondo argomento
(0.25) e' la frazione di round_interval_ms a cui far crashare ogni nodo. Entrambi sono
opzionali (default: 5 nodi, frazione 0.25).
