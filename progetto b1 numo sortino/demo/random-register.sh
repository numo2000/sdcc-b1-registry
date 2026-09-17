#!/bin/bash
# Registra NUM_SERVICES servizi su nodi scelti a caso del cluster, per
# mostrare durante la demo che la registrazione funziona da qualunque nodo
# (decentralizzazione) e che i dati si ritrovano discoverabili da nodi
# diversi da quello di registrazione (propagazione via gossip).
#
# Gli endpoint puntano al container demo-clients (vedi
# docker-compose.demo-clients.yml), che apre CLIENT_PORT_COUNT porte TCP in
# ascolto reale: piu' servizi diversi convergono sulla stessa porta (un
# "client" puo' benissimo fornire piu' servizi), cosi' il controllo di
# raggiungibilita' del registry li marca davvero ACTIVE invece che
# SUSPECTED - a differenza di endpoint fasulli mai raggiungibili.
#
# Uso: ./demo/random-register.sh [NUM_SERVICES] [NODE_COUNT] [CLIENT_PORT_COUNT]
set -e

NUM_SERVICES="${1:-8}"
NODE_COUNT="${2:-10}"
CLIENT_PORT_COUNT="${3:-10}"

# Porta host di ciascun nodo: 8080+N, con due eccezioni per porte gia'
# occupate (vedi generate-compose.sh, stessa logica).
node_port() {
	local n="$1"
	case "$n" in
		8) echo 8098 ;;
		18) echo 8121 ;;
		*) echo $((8080 + n)) ;;
	esac
}

echo "Registrazione di $NUM_SERVICES servizi su $NODE_COUNT nodi (nodo scelto a caso ad ogni registrazione)..."
echo

for i in $(seq 1 "$NUM_SERVICES"); do
	node=$(( (RANDOM % NODE_COUNT) + 1 ))
	port=$(node_port "$node")
	service_id="demo-svc-$i"
	client_port=$(( 9000 + ((i - 1) % CLIENT_PORT_COUNT) + 1 ))
	endpoint="demo-clients:$client_port"

	echo "-> registro $service_id su registry-node-$node (porta $port), endpoint $endpoint"
	curl -s -m 5 -X POST "http://localhost:$port/services" \
		-H "Content-Type: application/json" \
		-d "{\"service_id\":\"$service_id\",\"endpoint\":\"$endpoint\"}" \
		-o /dev/null -w "   HTTP %{http_code}\n" || echo "   TIMEOUT/ERRORE (nodo $node forse ancora in avvio)"

	sleep 0.3
done

echo
echo "Fatto. Verifica su un nodo diverso da quelli usati sopra, es.:"
echo "  curl http://localhost:8081/services | jq"
