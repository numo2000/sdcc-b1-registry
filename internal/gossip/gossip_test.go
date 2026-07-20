package gossip

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sdcc-b1-registry/internal/membership"
	"sdcc-b1-registry/internal/registry"
)

func newTestService(reg *registry.Registry) (*Service, *httptest.Server) {
	mem := membership.New(3*time.Second, 10*time.Second)
	svc := New("placeholder", reg, mem, 1, time.Second)

	mux := http.NewServeMux()
	mux.HandleFunc(ExchangePath, svc.ExchangeHandler)
	server := httptest.NewServer(mux)

	svc.SelfAddr = server.Listener.Addr().String()
	return svc, server
}

func TestDoRoundPropagatesEntriesBothWays(t *testing.T) {
	regA := registry.New("node-a")
	regA.Register("svc-a", "10.0.0.1:1", nil)
	svcA, serverA := newTestService(regA)
	defer serverA.Close()

	regB := registry.New("node-b")
	regB.Register("svc-b", "10.0.0.2:2", nil)
	svcB, serverB := newTestService(regB)
	defer serverB.Close()
	_ = svcB

	svcA.Membership.Seed([]string{serverB.Listener.Addr().String()})

	svcA.doRound()

	if _, ok := regA.Discover("svc-b"); !ok {
		t.Errorf("expected node A to learn svc-b from node B via gossip")
	}
	if _, ok := regB.Discover("svc-a"); !ok {
		t.Errorf("expected node B to learn svc-a from node A via the same push-pull exchange")
	}
}

func TestDoRoundLearnsSenderAddressForDynamicJoin(t *testing.T) {
	regA := registry.New("node-a")
	svcA, serverA := newTestService(regA)
	defer serverA.Close()

	regB := registry.New("node-b")
	svcB, serverB := newTestService(regB)
	defer serverB.Close()

	svcA.Membership.Seed([]string{serverB.Listener.Addr().String()})
	svcA.doRound()

	// node-b non conosceva node-a in anticipo: deve impararlo dallo scambio.
	found := false
	for _, p := range svcB.Membership.Snapshot() {
		if p.Address == svcA.SelfAddr {
			found = true
		}
	}
	if !found {
		t.Errorf("expected node B to learn node A's address as a side effect of the exchange")
	}
}

func TestDoSweepRemovesEntriesOfDeadPeer(t *testing.T) {
	reg := registry.New("node-a")
	reg.Merge(registry.ServiceEntry{
		ServiceID: "svc-x",
		Endpoint:  "10.0.0.9:9",
		Version:   registry.Version{Counter: 1, OwnerNodeID: "peer-dead:7000"},
		Status:    registry.StatusActive,
	})
	mem := membership.New(1*time.Millisecond, 2*time.Millisecond)
	mem.Seed([]string{"peer-dead:7000"})
	svc := New("self", reg, mem, 1, time.Second)

	time.Sleep(5 * time.Millisecond)
	svc.doSweep()

	if _, ok := reg.Discover("svc-x"); ok {
		t.Errorf("expected svc-x to be removed once its owner peer is marked DEAD")
	}
}

func TestExchangeWithUnreachablePeerDoesNotPanic(t *testing.T) {
	reg := registry.New("node-a")
	mem := membership.New(3*time.Second, 10*time.Second)
	mem.Seed([]string{"127.0.0.1:1"}) // porta improbabile, connessione rifiutata
	svc := New("self", reg, mem, 1, 200*time.Millisecond)

	svc.doRound() // non deve fallire in modo fatale, solo loggare l'errore
}
