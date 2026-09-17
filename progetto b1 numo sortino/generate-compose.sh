#!/bin/bash
# Genera docker-compose.yml per un cluster di NODE_COUNT nodi (uso:
# ./generate-compose.sh 40). Ogni servizio nasce dalla STESSA immagine
# (build: .), differenziato solo dal proprio configs/nodeN.yaml montato via
# bind mount - vedi generate-configs.sh per generare quei file.
#
# Porta host di ciascun nodo: 8080+N, con due eccezioni per porte gia'
# occupate sull'host da altri processi/servizi:
#   - nodo 8:  8098 (la naturale 8088 e' occupata da un processo esterno)
#   - nodo 18: 8121 (la naturale 8098 collide con l'eccezione del nodo 8
#              sopra - va assegnata alla prima porta libera dopo il blocco
#              principale, invece di rinumerare a cascata tutti i nodi
#              successivi)
set -e

NODE_COUNT="${1:-10}"

node_port() {
	local n="$1"
	case "$n" in
		8) echo 8098 ;;
		18) echo 8121 ;;
		*) echo $((8080 + n)) ;;
	esac
}

out="docker-compose.yml"
{
	echo "# Cluster di $NODE_COUNT nodi registry, ciascuno nato dalla STESSA immagine"
	echo "# (build: .), differenziati solo dal proprio config.yaml montato via bind"
	echo "# mount (vedi configs/node1..${NODE_COUNT}.yaml). Docker Compose crea"
	echo "# automaticamente una rete bridge condivisa e vi collega tutti i servizi"
	echo "# definiti qui: i nodi si raggiungono tra loro per NOME DEL SERVIZIO"
	echo "# (risoluzione DNS interna alla rete bridge, es. \"registry-node-2:8000\""
	echo "# nei bootstrap.seeds di ogni configs/nodeN.yaml)."
	echo "#"
	echo "# Generato da generate-compose.sh - non modificare a mano, rigenerare con"
	echo "# ./generate-compose.sh $NODE_COUNT se serve un numero diverso di nodi."
	echo "#"
	echo "# Nota sulla restart policy: nessuna impostata (default \"no\") di proposito"
	echo "# - un nodo \"ucciso\" con \`docker kill\` deve restare morto finche' non lo"
	echo "# riavvii tu esplicitamente, cosi' si puo' osservare la finestra di"
	echo "# SUSPECTED/DEAD (timeout_suspect_ms/timeout_dead_ms) durante i test di"
	echo "# crash invece di vederlo ripartire subito da solo."
	echo
	echo "services:"
	for i in $(seq 1 "$NODE_COUNT"); do
		port=$(node_port "$i")
		echo
		echo "  registry-node-${i}:"
		echo "    build: ."
		echo "    container_name: registry-node-${i}"
		echo "    volumes:"
		echo "      - ./configs/node${i}.yaml:/app/configs/config.yaml:ro"
		echo "    ports:"
		echo "      - \"${port}:7000\""
	done
} > "$out"

echo "Generato docker-compose.yml con $NODE_COUNT nodi."
