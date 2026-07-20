# Ripasso teorico — Progetto B1 (Service Registry Distribuito, Decentralizzato e Fault-Tolerant)

> Documento di ripasso mirato alla progettazione del progetto B1 del corso Sistemi Distribuiti e Cloud Computing (prof. Valeria Cardellini, Tor Vergata, A.A. 2025/26). I concetti sono estratti **esclusivamente** dalle slide del corso (`DS_Introduction.pdf`, `DS_Architecture.pdf`, `DS_Consistency.pdf`, `DS_Synchronization.pdf`, `DS_Communication-MOM.pdf`), senza aggiunte o generalizzazioni non presenti nel materiale. Per ogni concetto è indicata la fonte e una nota su come si applica al service registry. Dove le slide non coprono un argomento atteso, questo è segnalato esplicitamente: andrà integrato con letteratura esterna.

---

## 1. Modelli di sistema distribuito

### Definizioni di sistema distribuito
- **van Steen & Tanenbaum**: "A distributed system is a collection of autonomous computing elements that appears to its users as a single coherent system". Gli elementi (nodi) sono autonomi (hardware o processi software) e devono collaborare affinché il sistema appaia coerente agli utenti.
  *Fonte: DS_Introduction.pdf, slide 8.*
- **Lamport**: "A distributed system is one in which the failure of a computer you didn't even know existed can render your own computer unusable" — enfasi sulla tolleranza ai guasti.
  *Fonte: DS_Introduction.pdf, slide 9.*
- **Applicazione a B1**: il service registry è esattamente un sistema distribuito nel senso di Lamport — deve restare operativo anche se un nodo (che magari un client non conosce nemmeno) crasha. Nessun single point of failure è ammesso dalla consegna.

### Caratteristiche distintive dei sistemi distribuiti
- **Concorrenza**: nei DS è un dato di fatto, non una scelta progettuale come nei sistemi centralizzati.
- **Assenza di clock globale**: molti clock fisici, non necessariamente sincronizzati.
- **Guasti parziali e indipendenti**: un sistema centralizzato fallisce completamente, uno distribuito fallisce "a pezzi", spesso per problemi di comunicazione; nascondere completamente i guasti parziali è in generale impossibile.
  *Fonte: DS_Introduction.pdf, slide 10.*
- **Applicazione a B1**: giustifica perché il registry deve gestire il fatto che alcuni nodi/servizi possano essere irraggiungibili senza che l'intero sistema si blocchi.

### Sistemi sincroni vs asincroni
- **DS sincrono**: (1) vincoli su tempo di esecuzione di ogni processo (limiti inferiore e superiore); (2) ogni messaggio è ricevuto entro un tempo limitato; (3) ogni processo ha un clock fisico con drift rate limitato e noto.
- **DS asincrono**: nessun vincolo su velocità di esecuzione, ritardo di trasmissione dei messaggi o drift dei clock.
  *Fonte: DS_Synchronization.pdf, slide 4.*
- **Applicazione a B1**: un service registry decentralizzato basato su gossip via rete (es. Internet/LAN, container Docker) va progettato assumendo un modello **asincrono** — non si possono garantire limiti temporali sulla consegna di messaggi o sul drift dei clock; per questo l'ordinamento degli eventi (versioning dello stato) non può basarsi sul tempo fisico ma richiede clock logici (vedi sezione 4).

### Modello di guasto (fail-stop) e concetti di dependability
- **Fail-stop model**: "failed component simply stops functioning without any additional erroneous behavior" — citato a proposito della replica dei dati per aumentare la fault tolerance: "under fail-stop model, if up to k of k+1 servers crash, at least one is alive and can be used".
  *Fonte: DS_Consistency.pdf, slide 1.*
- **Fault, error, failure** (catena causale fault → error → failure):
  - **Failure**: il sistema (o componente) non rispetta le proprie specifiche (es. crash, risultato errato); può essere a cascata.
  - **Error**: stato interno errato che può portare a un failure (es. valore errato in memoria).
  - **Fault**: causa di un errore (es. memoria danneggiata); può essere transiente, intermittente, permanente.
  *Fonte: DS_Introduction.pdf, slide 23.*
- **Strumenti di dependability**: fault prevention, fault tolerance (mascherare il guasto), fault removal, fault forecasting.
  *Fonte: DS_Introduction.pdf, slide 23.*
- **Metriche di dependability**: Availability A(t), Reliability R(t), MTTF (Mean Time To Failure), MTTR (Mean Time To Repair), MTBF = MTTF + MTTR. Availability = MTTF/(MTTF+MTTR). Availability ≠ Reliability (esempi espliciti nelle slide con sistema che va giù 1 ms/ora vs sistema fermo 2 settimane/anno).
  *Fonte: DS_Introduction.pdf, slide 20-22.*
- **Applicazione a B1**: questi concetti forniscono il vocabolario per la relazione scientifica (giustificare le scelte di fault tolerance, discutere availability del registry). Il modello fail-stop è quello ragionevole da assumere per i nodi del registry (un nodo crasha e basta, non produce comportamento arbitrario) — semplifica la failure detection.
- **Nota**: **le slide non trattano esplicitamente una tassonomia dei modelli di guasto tipo crash failure vs omission failure vs Byzantine failure come categoria a sé stante** (salvo un accenno a "Byzantine behavior" nell'algoritmo di Berkeley e a "malicious/faulty" peer in blockchain, DS_Architecture.pdf slide 47, e "malicious time servers" in DS_Synchronization.pdf slide 9). **Questo va integrato con letteratura esterna** (es. van Steen & Tanenbaum cap. 8) se si vuole discutere formalmente la tassonomia dei guasti oltre al fail-stop.

### Le "8 fallacie del distributed computing" (Peter Deutsch)
1. The network is reliable
2. Latency is zero
3. Bandwidth is infinite
4. The network is secure
5. The topology does not change
6. There is one administrator
7. Transport cost is zero
8. The network is homogeneous
*Fonte: DS_Introduction.pdf, slide 28-29.*
- **Applicazione a B1**: argomento utile per la relazione — giustifica esplicitamente il perché serve failure detection, retry, timeout, gestione dinamica della topologia (i nodi cambiano — "the topology does not change... as long as it stays in the test lab").

---

## 2. Architetture decentralizzate e P2P

### Architetture centralizzate vs decentralizzate vs ibride
- **Architetture centralizzate**: modello client-server, comunicazione request/reply spesso sincrona e bloccante, forte accoppiamento.
- **Architetture decentralizzate**: sistemi P2P.
- **Architetture ibride**: combinano elementi centralizzati e decentralizzati (es. super-peer network, BitTorrent tracker+swarm, blockchain "spesso ibrida in pratica").
  *Fonte: DS_Architecture.pdf, slide 14, 18, 44.*
- **Applicazione a B1**: il progetto richiede esplicitamente "no server centrale" → architettura P2P pura, non hybrid con super-peer/tracker.

### Sistemi P2P: caratteristiche chiave
- Peer **simmetrici** nei ruoli (agiscono sia da client che da server, "servent"), nodi autonomi ai margini della rete; può esistere un super-peer con ruolo arricchito.
- **Nessun controllo centralizzato**.
- **Altamente distribuiti**: scalano a centinaia di migliaia di nodi.
- **Altamente dinamici**: i nodi entrano/escono in ogni momento (**churn**).
- **Ridondanza dell'informazione** per garantire resilienza e disponibilità.
  *Fonte: DS_Architecture.pdf, slide 18.*
- **Applicazione a B1**: descrive esattamente i requisiti del registry — gestione dinamica di join/leave dei nodi (churn), nessun controllo centralizzato, ridondanza dello stato per resilienza.

### Sfide dei sistemi P2P (rilevanti per il registry)
- Eterogeneità delle risorse dei peer
- Scalabilità
- **Fault tolerance e resilienza al churn**: "peers join, leave, or fail randomly — require robust failure recovery and data redundancy"
- Performance (efficienza del routing, load balancing, self-organization)
- Free-riding, anonimato/privacy, trust/reputation, minacce di rete (sybil, DDoS)
  *Fonte: DS_Architecture.pdf, slide 19-20.*
- **Compiti principali di un nodo P2P**: 1) Bootstrap (come un nuovo peer scopre gli altri: configurazione statica, cache preesistenti, nodi noti); 2) Resource lookup; 3) Resource retrieval.
  *Fonte: DS_Architecture.pdf, slide 20.*
- **Applicazione a B1**: il "bootstrap" è direttamente il meccanismo di seed nodes/lista di configurazione previsto nella ROADMAP del progetto per la scoperta iniziale dei nodi.

### Overlay network
- **Overlay network**: rete logica che connette i peer costruita sopra la rete IP fisica; i link logici possono non corrispondere a connessioni fisiche; fornisce un servizio di localizzazione delle risorse tramite routing a livello applicativo.
  *Fonte: DS_Architecture.pdf, slide 21.*
- **Classificazione**: overlay **non strutturati** vs **strutturati**.
  *Fonte: DS_Architecture.pdf, slide 22.*

### Overlay non strutturati
- Costruiti su grafi casuali senza struttura di design; i peer si connettono con regole semplici e locali; nessun controllo sul posizionamento delle risorse; gestiscono bene reti molto dinamiche.
- Pro: facilità di manutenzione, alta resilienza. Contro: lookup inefficiente.
- **Classificazione per distribuzione dell'indice**: centralizzato (es. Napster — SPOF), **decentralizzato** (es. Gnutella), ibrido (semi-centralizzato via super-peer).
  *Fonte: DS_Architecture.pdf, slide 23-24.*
- **Tecniche di lookup decentralizzato**:
  - **Query flooding**: l'originatore invia la query ai vicini; ogni peer risponde se possiede la risorsa, altrimenti inoltra ai propri vicini; ottimizzazioni: TTL (decrementato a ogni hop) e query ID unico per evitare cicli. Costo di lookup: **O(N)**. Difetti: overhead di comunicazione, rischio DoS/black-hole, falsi negativi, topology mismatch.
  - **Random walk**: l'originatore invia la query a un solo vicino scelto casualmente, che la inoltra a sua volta; minor overhead ma lookup più lento; variante **k-random walks** (k cammini paralleli indipendenti).
  - **Gossiping**: citato esplicitamente qui come terza tecnica di lookup decentralizzato ("probabilistic message spreading among peers"), con trattazione completa rimandata a "upcoming lesson" — la trattazione completa si trova infatti in `DS_Communication-MOM.pdf` (vedi sezione 6).
  *Fonte: DS_Architecture.pdf, slide 24-27.*
- **Applicazione a B1**: query flooding e random walk sono alternative dirette al gossip per la propagazione dello stato/lookup nel registry; le slide stesse le confrontano in termini di costo (O(N) per flooding) e overhead — utile per motivare nella relazione la scelta del gossip rispetto ad alternative più semplici.

### Overlay strutturati e DHT
- **Overlay strutturati**: le query di lookup sono instradate secondo una struttura ben definita (anello, albero, ipercubo, griglia...); ogni peer conosce un sottoinsieme di altri peer secondo tale struttura. Obiettivi: migliorare scalabilità abbassando il costo di lookup, ridurre overhead di comunicazione, lookup efficiente per chiave esatta. Contro: join/leave più costosi, struttura da mantenere.
  *Fonte: DS_Architecture.pdf, slide 27-28.*
- **Distributed Hash Table (DHT)**: astrazione distribuita di una hash table convenzionale; mappa una chiave-risorsa (GUID, generato tramite funzione hash sicura) al peer responsabile tramite una metrica di distanza. API: `get(K)`, `put(K,V)`, `remove(K)`.
  *Fonte: DS_Architecture.pdf, slide 28-29.*
- **Consistent hashing**: tecnica che mappa risorse e nodi sullo stesso spazio identificativo (anello); ogni nodo gestisce un intervallo contiguo di chiavi; quando la DHT si ridimensiona (join/leave) il remapping delle chiavi è minimo; carico bilanciato tra i nodi. Usata in Amazon Dynamo, Cassandra, Discord, Memcached.
  *Fonte: DS_Architecture.pdf, slide 33.*
- **Chord**: nodi e risorse mappati su un anello via consistent hashing; ogni nodo gestisce le chiavi tra sé e il predecessore; instradamento tramite **finger table** (m righe, dove m = bit del GUID); lookup **O(log N)**; join/leave **O(log² N)**; a ogni nodo mantiene puntatori a successore/predecessore; tolleranza ai guasti tramite replica di (K,V) su R nodi successori e "successor list"; **rilevamento dei nodi falliti tramite timeout o meccanismi di heartbeat** durante join/leave e stabilizzazione periodica dell'anello.
  *Fonte: DS_Architecture.pdf, slide 32-38.*
- **Kademlia**: overlay strutturato basato su DHT con GUID a 160 bit (SHA-1); distanza **XOR**; ogni valore memorizzato sui k nodi con ID più vicino alla chiave; routing table organizzata in **k-bucket**; query parallele/asincrone; robusto al churn; lookup **O(log N)**. Usato in BitTorrent DHT, IPFS.
  *Fonte: DS_Architecture.pdf, slide 39-41.*
- **DHT: bilancio finale**: decentralizzazione migliora load balancing e fault tolerance; churn resta un problema; punti di forza: consistent hashing, scalabilità incrementale, replica per alta disponibilità, self-management, nessun SPOF né controllo centralizzato.
  *Fonte: DS_Architecture.pdf, slide 41.*
- **Applicazione a B1**: se il registry adotta un approccio strutturato (DHT-style) per assegnare la responsabilità dei servizi ai nodi, Chord/Kademlia sono riferimenti diretti; in particolare l'uso di **timeout/heartbeat per il rilevamento dei nodi falliti** durante la manutenzione dell'anello Chord è il precedente più vicino, nelle slide, a un meccanismo di failure detection. Tuttavia, per un progetto più semplice basato su gossip non strutturato (come suggerito dalla ROADMAP), la parte DHT/Chord/Kademlia è più che altro riferimento concettuale/comparativo per la relazione ("perché non ho usato un overlay strutturato").

### Architetture ibride (cenno)
- Super-peer network, BitTorrent (tracker + swarm), blockchain (decentralizzata per design ma spesso ibrida in pratica).
  *Fonte: DS_Architecture.pdf, slide 44-46.*
- **Nota**: la sezione blockchain (validazione, consenso, proof-of-work/stake) non è direttamente rilevante per un service registry e viene qui omessa in dettaglio.

---

## 3. Consistenza

### Perché replicare e problema della consistenza
- Motivazioni per la replica: aumentare disponibilità (calcolo esplicito: con probabilità di fallimento p per server e n server, disponibilità = 1 − pⁿ), aumentare fault tolerance (fail-stop model), migliorare performance/scalabilità.
  *Fonte: DS_Consistency.pdf, slide 1.*
- **Il problema fondamentale**: mantenere le repliche consistenti richiede garantire che tutte le operazioni in conflitto sulla stessa risorsa avvengano ovunque nello stesso ordine; garantire un ordinamento globale ha però un costo (sincronizzazione globale) che compromette la scalabilità → si **indebolisce** il modello di consistenza per evitare sincronizzazione globale.
  *Fonte: DS_Consistency.pdf, slide 2-3.*
- **Applicazione a B1**: è la motivazione teorica diretta per la scelta di eventual consistency nel registry: con nodi distribuiti geograficamente/in container e latenza non nulla, un modello di consistenza forte sarebbe troppo costoso.

### Modelli di consistenza data-centrica (dal più forte al più debole)
Strict → Linearizability → Sequential → Causal → **Eventual**
*Fonte: DS_Consistency.pdf, slide 5.*

- **Strict consistency**: qualsiasi read su x ritorna il valore della write più recente; richiede ordinamento temporale assoluto e clock fisico globale — modello ideale, difficile da implementare (il tempo tra istruzioni è trascurabile rispetto al tempo di comunicazione).
  *Fonte: DS_Consistency.pdf, slide 6-7.*
- **Linearizability**: ogni operazione sembra avvenire istantaneamente in un punto tra inizio e fine, come se esistesse una timeline globale; le repliche eseguono le operazioni secondo un ordine totale che preserva l'ordinamento reale nel tempo. Fornisce semantica single-copy. Implementarla su WAN comporta blocco fino a propagazione completa → costosa in latenza.
  *Fonte: DS_Consistency.pdf, slide 7-9.*
- **Sequential consistency**: il risultato di qualunque esecuzione è uguale a quello ottenuto se le operazioni di tutti i processi fossero eseguite secondo un ordine sequenziale, e le operazioni di ogni processo appaiono in tale sequenza nell'ordine del proprio programma. Più debole di linearizability (non richiede rispetto dell'ordine "reale" nel tempo, solo un ordine totale coerente col program order di ciascun processo). Implementabile con sequencer centralizzato o multicast totalmente ordinato decentralizzato.
  *Fonte: DS_Consistency.pdf, slide 9-11.*
- **Causal consistency**: le operazioni di write potenzialmente in relazione causa/effetto devono essere viste da tutti i processi nello stesso ordine; write concorrenti possono essere viste in ordine diverso da processi diversi. Relazioni causali: read seguita da write sullo stesso processo; write seguita da read dello stesso dato su processi diversi; proprietà transitiva. Perde l'illusione single-copy. Per implementarla serve tracciare le dipendenze causali (grafo di dipendenza oppure, più agevolmente, **vector clock**).
  *Fonte: DS_Consistency.pdf, slide 12-15.*
- **Eventual consistency**: modello rilassato adatto quando ci sono poche write concorrenti (o conflitti facilmente risolvibili) e prevalenza di letture; garantisce che, se non arrivano nuovi aggiornamenti, **tutte le letture eventualmente restituiranno l'ultimo valore aggiornato** — tutte le repliche convergono gradualmente entro una **inconsistency window**, la cui durata (senza guasti) dipende da: latenza di comunicazione, numero di repliche, carico di sistema. Popolarizzata dal CAP theorem. Usata in alcuni servizi di storage cloud e data store NoSQL. Pro: semplice ed economica da implementare, letture/scritture veloci su replica locale (es. uso da parte del DNS). Contro: nessuna illusione di copia singola, possibile inconsistenza (staleness) da write concorrenti che richiede **riconciliazione**.
  *Fonte: DS_Consistency.pdf, slide 16-17.*
- **Riconciliazione delle versioni divergenti**: strategie — **last write wins**; taggare i dati con **vector clock** come timestamp per catturare la causalità tra versioni diverse (usato ad es. da Cassandra); in alternativa, delega della risoluzione conflitti all'applicazione (es. Amazon Dynamo). Momento della riconciliazione: su read (Dynamo), su write, o asynchronous repair.
  *Fonte: DS_Consistency.pdf, slide 17.*
- **Applicazione a B1 (concetto centrale del progetto)**: il registry deve implementare **eventual consistency**: ogni nodo mantiene una vista locale dello stato (servizi registrati), il gossip propaga gli aggiornamenti, e in assenza di nuovi update tutte le repliche convergono. La riconciliazione tramite **versioning/timestamp** (vector clock o last-write-wins, come previsto nella ROADMAP) è il meccanismo diretto da implementare per risolvere conflitti quando due nodi propagano informazioni divergenti sullo stesso servizio.

### CAP Theorem
- Proposto da Brewer (2000), dimostrato formalmente da Gilbert e Lynch (2002). Qualsiasi sistema a dati condivisi in rete può garantire **al più due** delle tre proprietà: **Consistency** (tutti i client vedono la stessa vista anche in presenza di update), **Availability** (tutti i client trovano una replica dei dati anche in presenza di guasti), **Partition tolerance** (la proprietà del sistema regge anche se il sistema è partizionato).
  *Fonte: DS_Consistency.pdf, slide 18.*
- Le partizioni di rete si verificano realmente (outage router, cavi sottomarini tagliati, DNS non funzionante) → si vuole comunque **P**, quindi la scelta reale è tra **C** e **A**: sistema **CP** (rinuncia a disponibilità) o sistema **AP** (rinuncia a consistenza forte → modello di consistenza rilassato: eventual consistency).
  *Fonte: DS_Consistency.pdf, slide 19-20.*
- **Applicazione a B1**: il progetto richiede esplicitamente "no single point of failure" e tolleranza a guasti/partizioni → il registry è per design un sistema **AP** (Availability + Partition tolerance), a scapito della Consistency forte, con eventual consistency come compromesso. Questo è l'argomento chiave da mettere nella relazione per giustificare le scelte architetturali.

### ACID vs BASE (materiale segnalato dalle slide come "consigliato ma non trattato a lezione")
- **ACID** (Atomicity, Consistency, Isolation, Durability): approccio pessimistico, previene i conflitti; standard per DBMS relazionali (es. Postgres, MySQL, sistemi CA); consistenza forte, minore disponibilità durante i guasti, maggiore overhead/latenza.
- **BASE** (Basically Available, Soft state, Eventual consistency): approccio ottimistico, lascia avvenire i conflitti ma li rileva e risolve; sistema disponibile quasi sempre; soft state = la persistenza è responsabilità dello sviluppatore; eventualmente consistente; maggiore throughput/minore latenza; adatto a DS su larga scala, NoSQL, applicazioni web dove availability e scalabilità orizzontale contano più della consistenza stretta.
  *Fonte: DS_Consistency.pdf, slide 22-23.*
- **Applicazione a B1**: il service registry ricade nel modello **BASE**, non ACID — utile terminologia per la relazione.

### Protocolli di consistenza data-centrica (implementazioni)
- **Protocolli primary-based** (o primary-backup/leader-based): a ogni dato è associata una replica primaria (leader) che coordina le scritture sulle repliche secondarie (follower); le letture possono avvenire su qualunque replica.
  - **Remote-write bloccante (sincrono)**: il client attende conferma da tutte le repliche → modello di consistenza **linearizability**; più tollerante ai guasti ma più lento.
  - **Remote-write non bloccante (asincrono)**: il client riceve conferma solo dalla primaria → modello **sequenziale**; più veloce, meno tollerante ai guasti, perde la linearizability.
  *Fonte: DS_Consistency.pdf, slide 24-26.*
- **Protocolli replicated-write**: scritture eseguite su più repliche senza controllo centralizzato.
  - **Replicazione attiva** (multi-leader): scrittura in multicast a tutte le repliche; il problema da risolvere è l'ordine di scrittura sulle repliche — richiede **multicasting totalmente ordinato** (sequencer centralizzato, oppure decentralizzato con clock scalare, oppure protocollo di consenso distribuito come Raft).
  - **Protocolli quorum-based**: con N repliche, un quorum di lettura NR e uno di scrittura NW; condizioni: NR+NW > N (evita conflitti read-write) e NW > N/2 (evita conflitti write-write, un solo scrittore alla volta può ottenere il quorum). Setting tipici: **ROWA** (Read Once Write All, NR=1, NW=N: letture veloci, scritture lente), **RAWO** (Read All Write Once, NW=1, NR=N: scritture veloci, letture lente, possibili conflitti write-write), **Majority** (NW=NR=N/2+1: entrambe relativamente lente ma alta disponibilità). Usato ad es. da Cassandra, con quorum configurabili per scegliere tra consistenza forte ed eventuale.
  *Fonte: DS_Consistency.pdf, slide 27-32.*
- **Applicazione a B1**: se il registry adotta repliche attive con multicast/gossip senza leader, questo è il framework replicated-write, non primary-based. I quorum non sono strettamente necessari per un registry eventually-consistent, ma sono un possibile refinement (es. "quorum read" per letture più affidabili) da citare nella relazione come alternativa considerata.

---

## 4. Sincronizzazione e clock logici

### Perché il tempo fisico non basta
- In un DS non è possibile un singolo clock fisico comune a tutti i processi; per molti algoritmi distribuiti è cruciale determinare almeno l'**ordinamento** degli eventi.
  *Fonte: DS_Synchronization.pdf, slide 2.*
- Timestamping basato solo sul clock fisico locale di ciascun processo permette di ricostruire l'ordine sullo stesso nodo, ma non tra nodi diversi.
  *Fonte: DS_Synchronization.pdf, slide 3.*
- **Sincronizzazione dei clock fisici** (Cristian's algorithm, Berkeley algorithm, NTP, Google TrueTime) è trattata dalle slide ma è rilevante principalmente per sistemi **sincroni** o a bassa latenza/LAN; in un DS asincrono la sincronizzazione fisica ha accuratezza limitata dovuta ai tempi di trasmissione variabili, e non si può usare il tempo fisico per ordinare eventi su nodi diversi.
  *Fonte: DS_Synchronization.pdf, slide 9-16, 30.*
- **Applicazione a B1**: la sincronizzazione dei clock fisici (NTP, Cristian, Berkeley) **non è la soluzione adatta** per versionare lo stato del registry (il progetto è un DS asincrono su nodi potenzialmente distribuiti); le slide stesse indicano il clock logico come alternativa per questo scenario. Questi algoritmi vanno citati nella relazione solo come contesto/confronto, non come meccanismo scelto.

### Happened-before relation (Lamport)
- Due eventi e, e' sono in relazione happened-before (e→e') se: (1) accadono sullo stesso processo nell'ordine osservato; (2) e è l'invio di un messaggio ed e' è la corrispondente ricezione; (3) la relazione è transitiva.
- Definisce un **ordinamento parziale**; eventi non collegati da happened-before sono **concorrenti** (e || e').
  *Fonte: DS_Synchronization.pdf, slide 17.*

### Scalar (Lamport) clock
- Contatore software monotono crescente per ogni processo; non si basa sul clock fisico.
- **Proprietà chiave**: se e→e' allora L(e) < L(e'). Il viceversa **non vale**: L(e) < L(e') non implica e→e' (limite del clock scalare: non permette di distinguere eventi concorrenti da eventi causalmente ordinati).
- **Algoritmo di aggiornamento**: init Li=0; evento interno → Li++; invio messaggio → Li++ poi timestamp allegato; ricezione messaggio con timestamp t → Lj = max(t, Lj) poi Lj++.
- **Ordinamento totale**: si aggiunge il numero di processo come tie-breaker per ottenere un ordine totale (e ⇒ e').
  *Fonte: DS_Synchronization.pdf, slide 18-20.*

### Vector clock
- Introdotto da Mattern (1989) e Fidge (1991) per superare il limite del clock scalare: il vector clock cattura **completamente** la relazione happened-before: e→e' se e solo se V(e) < V(e').
- Vettore di N interi (N = numero di processi); Vi[i] = contatore locale di pi; Vi[j] (j≠i) = numero di eventi di pj che pi conosce.
- Confronto: V=V' se uguali componente per componente; V<V' se tutte le componenti ≤ e almeno una strettamente <; V||V' (concorrenti) se né V<V' né V'<V.
- **Algoritmo di aggiornamento**: analogo allo scalare ma su ricezione si fa il max componente per componente (Vj[k]=max(t[k], Vj[k]) per ogni k) poi si incrementa la propria componente.
  *Fonte: DS_Synchronization.pdf, slide 21-23.*
- **Applicazione a B1 (concetto centrale)**: il **vector clock è lo strumento indicato dalle slide stesse (sezione "Timestamps in practice") per il caso d'uso di Amazon DynamoDB — "determine which object version is the most recent"** — esattamente il problema di versionare lo stato del service registry tra nodi che ricevono aggiornamenti via gossip in ordine diverso. È il meccanismo più adatto, tra quelli visti a lezione, per riconciliare versioni concorrenti (vedi anche eventual consistency, sezione 3).

### Multicast totalmente ordinato e causalmente ordinato
- **Totally ordered multicast**: tutti i messaggi sono consegnati nello stesso ordine a tutti i destinatari. Soluzioni: centralizzata (sequencer — semplice ma SPOF/collo di bottiglia) o decentralizzata con clock scalare (ogni processo mette in coda i messaggi ordinati per timestamp e consegna solo quando è sicuro che nessun altro processo può inviare un messaggio con timestamp minore o uguale — richiede ack O(N²)).
  *Fonte: DS_Synchronization.pdf, slide 24-26.*
- **State machine replication (SMR)**: tecnica per cui ogni replica esegue la stessa sequenza di operazioni nello stesso ordine, restando sincronizzata; il servizio progredisce finché una maggioranza delle repliche è up; si può recuperare riapplicando le operazioni nello stesso ordine. Alternative più scalabili al multicast totalmente ordinato di Lamport: **Paxos** e **Raft**.
  *Fonte: DS_Synchronization.pdf, slide 27-28.*
- **Causally ordered multicast**: un messaggio è consegnato solo dopo che tutti i messaggi che lo precedono causalmente sono stati consegnati; più debole del totally ordered; implementato con vector clock: il messaggio viene messo in coda d'attesa finché non risultano soddisfatte le condizioni sul vector timestamp rispetto al proprio vettore locale.
  *Fonte: DS_Synchronization.pdf, slide 28-29.*
- **Applicazione a B1**: il registry non necessita di ordinamento totale stretto (troppo costoso, richiede sequencer o consenso) — l'obiettivo è **eventual consistency**, quindi al più causal ordering (via vector clock) per gli aggiornamenti collegati, mentre gli aggiornamenti concorrenti possono essere riconciliati con last-write-wins.

---

## 5. Failure detection

**Le slide del corso analizzate NON contengono una trattazione sistematica/dedicata della failure detection** (non c'è uno slide set specifico su heartbeat, timeout, phi-accrual failure detector, SWIM, ecc., nei 5 PDF esaminati). I riferimenti presenti sono solo incidentali, elencati di seguito — **questa è l'area più carente e va integrata con letteratura esterna** (es. paper sul phi-accrual failure detector, algoritmo SWIM, o il libro di van Steen & Tanenbaum cap. 8 sulla fault tolerance).

Riferimenti incidentali trovati nelle slide:
- **Chord**: "Nodes may leave the ring abruptly due to failure — nodes must detect failed nodes, **using timeouts or heartbeat mechanisms**"; ogni nodo esegue periodicamente un **ring stabilization protocol** per aggiornare le finger table; per la fault tolerance, Chord replica le coppie (K,V) sui successori e mantiene una "successor list"; al recovery da crash un nodo verifica con successori/predecessori eventuali aggiornamenti mancati.
  *Fonte: DS_Architecture.pdf, slide 37-38.*
- **Berkeley algorithm** (sincronizzazione clock, non failure detection generico ma un caso applicativo): se il master fallisce, viene eletto un nuovo master tramite un algoritmo di elezione; il master ignora/scarta valori di clock troppo distanti dalla media (outlier discarding) per tollerare comportamenti Byzantine dei worker.
  *Fonte: DS_Synchronization.pdf, slide 11-12.*
- **NTP**: la synchronization subnet si riconfigura automaticamente in caso di guasti (un server primario che perde la connessione alla sorgente UTC diventa secondario; un secondario che perde il primario passa a un altro primario).
  *Fonte: DS_Synchronization.pdf, slide 13.*
- **Gossip per failure detection** (menzione esplicita, senza dettaglio algoritmico): "Amazon's Dynamo uses gossiping for **node failure detection**"; "Cassandra uses gossiping for **group membership and node failure detection**"; tra i domini applicativi del gossip: "Resource management in large-scale DS — including monitoring and **failure detection**".
  *Fonte: DS_Communication-MOM.pdf, slide 42 ("Who uses gossiping?") e slide 47 ("Other application domains of gossiping").*
- **Kafka/ISR (In-Sync Replicas)**: se il leader di una partizione crasha, un follower in-sync viene eletto nuovo leader — meccanismo di failure detection e failover a livello di sistema concreto (non teoria generale).
  *Fonte: DS_Communication-MOM.pdf, slide 47.*

**Cosa manca e va cercato altrove**: definizione formale di failure detector (completeness/accuracy), algoritmi heartbeat/ping-ack espliciti con pseudocodice, phi-accrual failure detector (usato realmente da Cassandra e Akka), algoritmo SWIM (usato da Serf/Consul, spesso citato come riferimento canonico per registry gossip-based). Dato che il progetto B1 richiede esplicitamente un meccanismo di failure detection (ping/heartbeat + timeout, come da ROADMAP), questa è la parte teorica da approfondire con fonti esterne al corso.

---

## 6. Comunicazione: Message-Oriented Middleware e Gossip Protocol

### Message-oriented communication e decoupling
- Il paradigma a messaggi migliora il **decoupling** rispetto a RPC (che ha accoppiamento temporale e spaziale). Tre tipi di decoupling: **spaziale** (i componenti non devono conoscersi), **temporale** (non devono essere presenti contemporaneamente), **di sincronizzazione** (non si bloccano a vicenda).
  *Fonte: DS_Architecture.pdf, slide 9; DS_Communication-MOM.pdf, slide 1.*
- **MOM (Message Oriented Middleware)**: comunicazione persistente, storage intermedio dei messaggi, loose coupling; pattern: **message queue** (one-to-one) e **publish-subscribe** (one-to-many).
  *Fonte: DS_Communication-MOM.pdf, slide 2.*
- **Applicazione a B1**: utile come contesto per motivare perché un registry gossip-based (comunicazione asincrona, disaccoppiata) è preferibile a RPC sincrono tra tutti i nodi.

### Semantiche di delivery (rilevanti per la progettazione dei messaggi di gossip)
- **At-most-once**: il messaggio può andare perso ma non è mai riconsegnato più volte.
- **At-least-once**: nessuna perdita ma possibili duplicati (richiede ack e ritrasmissione da parte del MOM se manca l'ack).
- **Exactly-once**: nessuna perdita né duplicati; richiede deduplicazione (ID univoco per messaggio) e/o consumer idempotenti.
- **Transaction-based**: il messaggio è rimosso dalla coda solo dopo elaborazione riuscita (commit/rollback).
- **Timeout-based**: il messaggio diventa invisibile finché non scade un timeout di visibilità; se non arriva l'ack, ridiventa visibile per un altro consumer (usato da Amazon SQS).
  *Fonte: DS_Communication-MOM.pdf, slide 6-8.*
- **Applicazione a B1**: utile terminologia per descrivere nella relazione la semantica di consegna scelta per i messaggi di gossip tra nodi (tipicamente **at-least-once** è la scelta naturale per un protocollo gossip, dato che i messaggi ridondanti sono tollerati e anzi contribuiscono alla robustezza).

### Gossip-based protocols (sezione più direttamente rilevante per B1)
- **Definizione**: protocolli probabilistici (aka **epidemic algorithms**); l'informazione si diffonde nel gruppo come in un'epidemia; ogni nodo invia il messaggio a un **sottoinsieme scelto casualmente** di nodi noti, che a loro volta lo ritrasmettono a un sottoinsieme casuale, e così via.
  *Fonte: DS_Communication-MOM.pdf, slide 41.*
- **Origine**: proposti nel 1987 da Demers et al. per la consistenza dei dati in database replicati con centinaia di server, assumendo assenza di conflitti in scrittura; le repliche condividono lo stato aggiornato solo con pochi vicini selezionati; la propagazione è **lazy** (non immediata); ogni update dovrebbe **eventualmente** raggiungere ogni replica.
  *Fonte: DS_Communication-MOM.pdf, slide 41 ("Origin of gossip-based protocols").*
- **Proprietà attrattive per DS su larga scala**: semplicità, nessun controllo/gestione centralizzata (né relativo collo di bottiglia), **scalabilità** (ogni nodo invia un numero limitato di messaggi indipendentemente dalla dimensione del sistema), **affidabilità e robustezza** grazie alla ridondanza dei messaggi.
  *Fonte: DS_Communication-MOM.pdf, slide 42.*
- **Chi usa il gossip (esempi citati esplicitamente)**: AWS S3 (per instradare intorno a server falliti/irraggiungibili), Amazon Dynamo (**node failure detection**), BitTorrent, **Cassandra (group membership e node failure detection)**.
  *Fonte: DS_Communication-MOM.pdf, slide 42.*

#### Strategie di diffusione degli aggiornamenti
- **Anti-entropy**: ogni nodo seleziona periodicamente un altro nodo a caso e scambia gli aggiornamenti (differenze di stato) con l'obiettivo di rendere identici gli stati dei due nodi. Varianti: **push** (P spinge i propri aggiornamenti a Q), **pull** (P tira gli aggiornamenti da Q), **push-pull** (scambio in entrambe le direzioni). Push-pull è la strategia più veloce: O(ln N) round per diffondere un aggiornamento a N nodi, con O(N ln ln N) messaggi; un **round (gossip cycle)** è l'intervallo di tempo in cui ogni nodo avvia uno scambio.
  *Fonte: DS_Communication-MOM.pdf, slide 43-44.*
- **Rumor spreading**: un nodo con un aggiornamento nuovo ("contaminato") seleziona periodicamente F (F≥1) peer e invia loro l'update ("contaminandoli"); un nodo che riceve un update già noto può, con probabilità p_stop, smettere di propagarlo. Combinabile con anti-entropy per migliorare la diffusione quando p_stop è alto.
  *Fonte: DS_Communication-MOM.pdf, slide 44.*
- **Framework generale di un protocollo di gossip** (Kermarrec & van Steen, 2007): a ogni round, un nodo P: `selectPeer` (seleziona un nodo Q casualmente), `selectToSend` (seleziona un sottoinsieme della propria vista locale da inviare), scambio con Q, `selectToKeep` (seleziona quali entry ricevute mantenere nella vista locale, rimuovendo duplicati), `processData`. Aspetti cruciali: come selezionare il peer (es. uniformemente tra i nodi vivi disponibili, oppure selezionando il peer meno contattato di recente — usato ad es. da CockroachDB), cosa scambiare (dipende dall'applicazione), strategia di update, elaborazione dei dati.
  *Fonte: DS_Communication-MOM.pdf, slide 45-46.*
- **Applicazione a B1 (nucleo del progetto)**: questo è il framework teorico diretto per il **loop di gossip periodico** richiesto dalla ROADMAP (Fase 3). Consigli operativi:
  - Scegliere **push-pull** come strategia di anti-entropy per convergenza rapida con overhead contenuto.
  - Il "round" del framework corrisponde esattamente all'intervallo di gossip configurabile menzionato in ROADMAP.
  - `selectToKeep`/riconciliazione va implementato usando versioning (vector clock o last-write-wins, sezione 3-4) per decidere quale tra due versioni divergenti di un servizio mantenere.

#### Gossip vs flooding
- Il gossip è generalmente **più efficiente del flooding** in termini di messaggi scambiati (esempio numerico nelle slide: flooding 18 messaggi/8 nodi raggiunti su 9 in 3 round; rumor spreading probabilistico 11 messaggi/7 nodi raggiunti su 9 in 3 round).
- **Caratteristiche del gossip**: probabilistico, decisione localizzata che però porta a uno stato globale, leggero (lightweight), **fault-tolerant**.
- **Flooding**: copertura universale garantita e stato minimo richiesto, ma può inondare la rete di messaggi ridondanti.
- Il gossip riduce le trasmissioni ridondanti rispetto al flooding mantenendone i vantaggi, ma per la sua natura probabilistica **non garantisce** che tutti i peer siano raggiunti, e in genere impiega più tempo del flooding per completare la diffusione.
  *Fonte: DS_Communication-MOM.pdf, slide 46-47.*
- **Applicazione a B1**: argomento utile per la relazione — giustifica il trade-off scelto (gossip vs flooding) e permette di discutere onestamente i limiti (mancanza di garanzia di copertura totale), coerente con il fatto che il progetto richiede solo eventual, non strong, consistency.

#### Altri domini applicativi del gossip (oltre alla disseminazione dell'informazione)
- **Group membership**: conoscere la lista dei nodi che fanno parte del sistema.
- **Peer sampling**: selezionare nodi da un insieme più ampio di nodi disponibili con cui interagire.
- **Resource management in DS su larga scala**, incluso **monitoring e failure detection**.
- **Calcoli distribuiti/aggregazione dei dati** (es. reti di sensori): calcolo di somma, media, massimo, minimo — esempio esplicito nelle slide: durante uno scambio gossip, i nodi i e j scambiano i propri valori correnti v_i, v_j e li aggiornano a (v_i+v_j)/2.
  *Fonte: DS_Communication-MOM.pdf, slide 47.*
- **Applicazione a B1**: **group membership** e **peer sampling** sono i due domini più direttamente rilevanti — il registry deve mantenere una vista di quali nodi/servizi sono attivi (group membership) e, nella variante gossip, ogni nodo deve scegliere con chi "spettegolare" a ogni round (peer sampling). Le slide non forniscono però un algoritmo dettagliato di peer sampling (es. SCAMP, Cyclon) — da approfondire esternamente se si vuole un meccanismo di selezione dei peer più sofisticato del semplice random uniform.

#### Casi di studio di gossip (menzionati, meno centrali per B1)
- **Blind counter rumor mongering**: variante con due parametri B (numero massimo di vicini a cui inoltrare) e F (numero di volte che un nodo inoltra lo stesso messaggio); performance vs flooding: circa il 50% di messaggi in meno, ma copertura incompleta (~90%) e diffusione più lenta (~2x).
- **Bimodal multicast (pbcast)**: protocollo a due fasi — (1) distribuzione del messaggio senza garanzie particolari di affidabilità (multicast inaffidabile), (2) "gossip repair": ogni processo invia periodicamente un digest del proprio stato a un peer casuale, che confronta col proprio storico e richiede le copie dei messaggi mancanti. Proprietà "bimodale": il messaggio viene consegnato a quasi tutti o quasi nessuno (mai "a metà"); due distribuzioni di latenza (bassa per consegna diretta, più alta per consegna riparata). Usato da Fastly CDN per l'invalidazione di cache.
  *Fonte: DS_Communication-MOM.pdf, slide 48-51.*
- **Applicazione a B1**: questi sono case study avanzati, utili solo se si vuole approfondire la relazione con un confronto tra varianti di gossip; non indispensabili per l'implementazione base.

### Pub/sub event matching e architetture decentralizzate (cenno)
- Le slide discutono anche architetture di **event matching decentralizzate** per sistemi publish-subscribe (overlay non strutturato con flooding/gossiping per disseminare le notifiche, oppure overlay strutturato con DHT), come esempio di applicazione del gossip a un problema diverso dal semplice "message dissemination".
  *Fonte: DS_Communication-MOM.pdf, slide 53-54.*
- **Nota**: questa parte è più legata ai sistemi pub/sub in generale che al service registry; citata solo per completezza, non centrale per B1.

---

## Riepilogo: cosa è ben coperto dalle slide e cosa richiede integrazione esterna

| Area | Copertura nelle slide del corso | Nota |
|---|---|---|
| Modelli di sistema (sincrono/asincrono, van Steen/Lamport, dependability) | **Buona** (DS_Introduction, DS_Synchronization) | Manca una tassonomia esplicita dei modelli di guasto (crash/omission/Byzantine) oltre al fail-stop → **integrare esternamente** |
| Architetture P2P, overlay strutturati/non strutturati, DHT, Chord, Kademlia | **Ottima** (DS_Architecture) | Completa e direttamente applicabile |
| Gossip protocol | **Ottima** (DS_Communication-MOM, sezione dedicata estesa) | Framework, anti-entropy, rumor spreading, gossip vs flooding, casi d'uso reali (Dynamo, Cassandra, S3) tutti presenti |
| Consistenza (strict/linearizability/sequential/causal/eventual, CAP, ACID/BASE, quorum) | **Ottima** (DS_Consistency) | Completa; ACID vs BASE segnalato dalle slide stesse come materiale extra non trattato a lezione ma presente nei PDF |
| Clock logici (Lamport scalar, vector clock, happened-before, causal/totally ordered multicast) | **Ottima** (DS_Synchronization) | Completa e con applicazione esplicita citata (DynamoDB usa vector clock per il versioning — caso identico al registry) |
| Failure detection (heartbeat, timeout, phi-accrual, SWIM) | **Debole/frammentaria** | Solo cenni sparsi (Chord: timeout/heartbeat per la manutenzione dell'anello; menzioni che Dynamo/Cassandra "usano il gossip per la failure detection" senza spiegare l'algoritmo). **Nessun algoritmo di failure detection è trattato in dettaglio nelle slide esaminate → va integrato con letteratura esterna** (phi-accrual failure detector, algoritmo SWIM, o capitolo dedicato di van Steen & Tanenbaum) |
| Pub-sub / MOM in generale | **Buona** (DS_Communication-MOM) | Utile come contesto sul decoupling, meno centrale per l'implementazione core del registry |

**Conclusione operativa per lo studio**: le slide del corso coprono in modo solido tutti gli argomenti chiave per il progetto B1 — gossip, eventual consistency/CAP, vector clock per il versioning, architetture P2P decentralizzate — tranne la **failure detection vera e propria**, che va necessariamente integrata con fonti esterne al corso prima di progettare il meccanismo di rilevamento dei nodi crashati richiesto dalla consegna.
