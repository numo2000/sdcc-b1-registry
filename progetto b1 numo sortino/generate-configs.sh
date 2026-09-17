#!/bin/bash
# Genera configs/nodeN.yaml per un cluster di NODE_COUNT nodi.
#
# round_interval_ms e' sfasato per nodo per ridurre la probabilita' che nodi
# diversi contattino lo stesso peer nello stesso istante:
#
#   offset_ms(N) = ((N-1) * STEP_MS) mod CYCLE_MS
#   round_interval_ms(N) = BASE_MS + offset_ms(N)
#
# Lo sfasamento e' calcolato in modulo (CYCLE_MS), non linearmente su N: con
# un incremento lineare puro, un cluster di centinaia di nodi porterebbe
# l'ultimo nodo ad avere un round_interval_ms molto piu' alto del primo
# (es. +1990ms su 200 nodi, quasi il triplo della base) - non piu' un lieve
# sfasamento ma un ritmo di gossip effettivamente diverso da nodo a nodo. Con
# il modulo, lo spread resta limitato a CYCLE_MS indipendentemente dal
# numero di nodi: oltre CYCLE_MS/STEP_MS nodi, l'offset si ripete ciclicamente.
set -e

NODE_COUNT="${1:-10}"

BASE_MS=1000
STEP_MS=10
CYCLE_MS=200

PEERS_PER_ROUND=2
DEAD_CHECK_EVERY_N_ROUNDS=10
TIMEOUT_SUSPECT_MS=3000
TIMEOUT_DEAD_MS=10000
SERVICE_HEALTH_CHECK_INTERVAL_MS=5000
SERVICE_CHECK_TIMEOUT_MS=500

for i in $(seq 1 "$NODE_COUNT"); do
	offset=$(( ((i - 1) * STEP_MS) % CYCLE_MS ))
	round_interval=$(( BASE_MS + offset ))

	out="configs/node${i}.yaml"
	{
		echo "# Configurazione del nodo $i nel cluster a $NODE_COUNT nodi (generato da generate-configs.sh)."
		echo
		echo "node:"
		echo "  id: \"registry-node-${i}:8000\""
		echo "  listen_port: 7000"
		echo "  grpc_port: 8000"
		echo
		echo "bootstrap:"
		echo "  seeds:"
		for j in $(seq 1 "$NODE_COUNT"); do
			if [ "$j" -ne "$i" ]; then
				echo "    - \"registry-node-${j}:8000\""
			fi
		done
		echo
		echo "gossip:"
		echo "  round_interval_ms: ${round_interval}"
		echo "  peers_per_round: ${PEERS_PER_ROUND}"
		echo "  dead_check_every_n_rounds: ${DEAD_CHECK_EVERY_N_ROUNDS}"
		echo
		echo "failure_detection:"
		echo "  timeout_suspect_ms: ${TIMEOUT_SUSPECT_MS}"
		echo "  timeout_dead_ms: ${TIMEOUT_DEAD_MS}"
		echo
		echo "service_health:"
		echo "  check_interval_ms: ${SERVICE_HEALTH_CHECK_INTERVAL_MS}"
		echo "  check_timeout_ms: ${SERVICE_CHECK_TIMEOUT_MS}"
	} > "$out"
done

echo "Generati $NODE_COUNT file in configs/ (round_interval_ms sfasato in modulo ${CYCLE_MS}ms, step ${STEP_MS}ms)."
