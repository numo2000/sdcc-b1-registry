#!/bin/bash
# Test 1: registrazione/rimozione dinamica con fotografia dell'evoluzione.
# Presuppone il cluster gia' avviato (docker compose up -d) su localhost.
# Uso: bash scripts/test1_addremove_scan.sh [numero_nodi]
set -e

N=${1:-5}
OUT=/tmp/test1_scan.csv

node_port() {
	case "$1" in
		8) echo 8098 ;;
		18) echo 8121 ;;
		*) echo $((8080 + $1)) ;;
	esac
}

t0=$(date +%s.%N)
elapsed_ms() { awk -v t0="$t0" -v now="$(date +%s.%N)" 'BEGIN{printf "%d", (now-t0)*1000}'; }

# ritorna il numero di servizi ACTIVE noti al nodo N (senza dipendere da jq)
active_count() {
	local n="$1"
	local p
	p=$(node_port "$n")
	curl -s -m 3 "http://localhost:$p/services" 2>/dev/null \
		| grep -o '"status":"ACTIVE"' | wc -l
}

scan_row() {
	local label="$1"
	local ms
	ms=$(elapsed_ms)
	local row="$label;$ms"
	local line="  ${label} (t+${ms}ms):"
	for i in $(seq 1 "$N"); do
		local c
		c=$(active_count "$i")
		row="$row;$c"
		line="$line node-$i=$c"
	done
	echo "$row" >> "$OUT"
	echo "$line"
}

echo "scan;elapsed_ms" > "$OUT"
header="scan;elapsed_ms"
for i in $(seq 1 "$N"); do header="$header;node-$i"; done
echo "$header" > "$OUT"

echo "=== Registro un servizio per nodo (S1..S$N), uno alla volta ==="
for i in $(seq 1 "$N"); do
	p=$(node_port "$i")
	curl -s -m 3 -o /dev/null -X POST "http://localhost:$p/services" \
		-H "Content-Type: application/json" \
		-d "{\"service_id\":\"S$i\",\"endpoint\":\"registry-node-$(( (i % N) + 1 )):7000\"}"
	sleep 0.2
done
scan_row "registrazione_appena_fatta"
sleep 1.2
scan_row "registrazione_convergente"

echo "=== Rimuovo S1 (solo node-1) ==="
p1=$(node_port 1)
curl -s -m 3 -o /dev/null -X DELETE "http://localhost:$p1/services/S1"
scan_row "rimozione_immediata"
sleep 0.4
scan_row "rimozione_propagazione"
sleep 0.6
scan_row "rimozione_convergente"

echo "=== Aggiungo S_new (solo node-2) ==="
p2=$(node_port 2)
curl -s -m 3 -o /dev/null -X POST "http://localhost:$p2/services" \
	-H "Content-Type: application/json" -d '{"service_id":"S_new","endpoint":"registry-node-1:7000"}'
scan_row "aggiunta_immediata"
sleep 0.4
scan_row "aggiunta_propagazione"
sleep 0.6
scan_row "aggiunta_convergente"

echo
echo "=== Risultato completo (anche in $OUT) ==="
column -t -s';' "$OUT"

echo
echo "Pulizia: rimuovo i servizi di prova..."
for i in $(seq 1 "$N"); do
	p=$(node_port "$i")
	curl -s -m 3 -o /dev/null -X DELETE "http://localhost:$p/services/S$i" 2>/dev/null || true
done
curl -s -m 3 -o /dev/null -X DELETE "http://localhost:$p2/services/S_new" 2>/dev/null || true
echo "fatto."
