#!/bin/bash
# Test 2: crash del nodo a una frazione del proprio round_interval_ms,
# classificazione push/pull dai log. Versione leggera per demo dal vivo:
# un solo trial per nodo (non 4 frazioni x N nodi come nella relazione).
# Presuppone il cluster gia' avviato (docker compose up -d) su localhost.
# Uso: bash scripts/test2_crash_pushpull.sh [numero_nodi] [frazione]
set -e

N=${1:-5}
FRAC=${2:-0.25}
OUT=/tmp/test2_pushpull.csv

node_port() {
	case "$1" in
		8) echo 8098 ;;
		18) echo 8121 ;;
		*) echo $((8080 + $1)) ;;
	esac
}

echo "trial;node;round_interval_ms;fraction;crash_delay_ms;service_id;push_happened;pull_happened;final_survived" > "$OUT"

for n in $(seq 1 "$N"); do
	ri=$(( 1000 + (10 * (n - 1)) % 200 ))
	p=$(node_port "$n")
	svc="crashdemo-n${n}"
	delay_ms=$(awk -v ri="$ri" -v f="$FRAC" 'BEGIN{printf "%d", ri*f}')
	delay_s=$(awk -v d="$delay_ms" 'BEGIN{printf "%.3f", d/1000}')

	t0=$(date -u +"%Y/%m/%d %H:%M:%S.%6N")
	curl -s -m 3 -o /dev/null -X POST "http://localhost:$p/services" \
		-H "Content-Type: application/json" \
		-d "{\"service_id\":\"$svc\",\"endpoint\":\"registry-node-$(( (n % N) + 1 )):7000\"}"

	sleep "$delay_s"
	t1=$(date -u +"%Y/%m/%d %H:%M:%S.%6N")
	echo "node-$n: registrato $svc, crash dopo ${delay_ms}ms (frazione $FRAC di round_interval_ms=${ri}ms)"
	docker kill "registry-node-$n" > /dev/null 2>&1

	sleep 1
	docker start "registry-node-$n" > /dev/null 2>&1
	sleep 1

	# estrai i log di tutti i nodi e classifica push/pull nella finestra [t0,t1]
	push="no"
	docker logs "registry-node-$n" > "/tmp/t2_log_$n.txt" 2>&1 || true
	if awk -v t0="$t0" -v t1="$t1" 'BEGIN{f=0} /gossip: scambio con .* riuscito/{ts=substr($0,1,26); if (ts>=t0 && ts<=t1) f=1} END{exit !f}' "/tmp/t2_log_$n.txt"; then
		push="yes"
	fi

	pull="no"
	target="scambio con registry-node-${n}:8000 riuscito"
	for m in $(seq 1 "$N"); do
		if [ "$m" != "$n" ]; then
			docker logs "registry-node-$m" > "/tmp/t2_log_$m.txt" 2>&1 || true
			if awk -v t0="$t0" -v t1="$t1" -v tgt="$target" 'BEGIN{f=0} { ts=substr($0,1,26); if (ts>=t0 && ts<=t1 && index($0,tgt)>0) f=1 } END{exit !f}' "/tmp/t2_log_$m.txt"; then
				pull="yes"
				break
			fi
		fi
	done

	checknode=1
	[ "$n" -eq 1 ] && checknode=2
	cp=$(node_port "$checknode")
	code=$(curl -s -m 3 -o /dev/null -w "%{http_code}" "http://localhost:$cp/services/$svc" 2>/dev/null || echo "000")
	survived="no"
	[ "$code" = "200" ] && survived="yes"

	echo "  -> push=$push pull=$pull sopravvissuto=$survived"
	echo "$n;node-$n;$ri;$FRAC;$delay_ms;$svc;$push;$pull;$survived" >> "$OUT"
done

echo
echo "=== Risultato completo (anche in $OUT) ==="
column -t -s';' "$OUT"
