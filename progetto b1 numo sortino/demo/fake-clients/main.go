// Programma di supporto per la demo: apre PORT_COUNT porte TCP in ascolto
// (accept+close, nessuna logica applicativa) per simulare "client" reali e
// raggiungibili a cui far puntare i servizi fasulli registrati con
// demo/random-register.sh. Serve solo a dare al controllo di
// raggiungibilita' del registry (gossip.Service.checkReachable) degli
// endpoint TCP genuinamente vivi da poter marcare ACTIVE - non fa parte del
// sistema distribuito vero e proprio.
package main

import (
	"flag"
	"log"
	"net"
	"strconv"
)

func main() {
	basePort := flag.Int("base-port", 9001, "prima porta della sequenza di ascolto")
	count := flag.Int("count", 10, "numero di porte consecutive da aprire")
	flag.Parse()

	for i := 0; i < *count; i++ {
		port := *basePort + i
		go listenAndAccept(port)
	}

	log.Printf("fake-clients: in ascolto su %d porte a partire da %d", *count, *basePort)
	select {} // il processo resta vivo per sempre, i listener girano in background
}

func listenAndAccept(port int) {
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalf("fake-clients: impossibile aprire la porta %d: %v", port, err)
	}
	for {
		conn, err := lis.Accept()
		if err != nil {
			return
		}
		conn.Close()
	}
}
