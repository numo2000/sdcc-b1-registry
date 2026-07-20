package membership

import (
	"testing"
	"time"
)

// fakeClock permette di avanzare il tempo deterministicamente nei test di
// failure detection, senza dover fare time.Sleep reali.
type fakeClock struct {
	t time.Time
}

func (f *fakeClock) now() time.Time { return f.t }
func (f *fakeClock) advance(d time.Duration) { f.t = f.t.Add(d) }

func newViewWithFakeClock(timeoutSuspect, timeoutDead time.Duration) (*View, *fakeClock) {
	v := New(timeoutSuspect, timeoutDead)
	clock := &fakeClock{t: time.Now()}
	v.now = clock.now
	return v, clock
}

func TestSeedStartsAlive(t *testing.T) {
	v, _ := newViewWithFakeClock(3*time.Second, 10*time.Second)
	v.Seed([]string{"peer-a:7000"})

	peers := v.Snapshot()
	if len(peers) != 1 || peers[0].State != StateAlive {
		t.Fatalf("expected seeded peer to start ALIVE, got %+v", peers)
	}
}

func TestSweepMarksSuspectedThenDead(t *testing.T) {
	v, clock := newViewWithFakeClock(3*time.Second, 10*time.Second)
	v.Seed([]string{"peer-a:7000"})

	clock.advance(4 * time.Second)
	suspected, dead := v.Sweep()
	if len(suspected) != 1 || suspected[0] != "peer-a:7000" {
		t.Fatalf("expected peer-a to become SUSPECTED, got suspected=%v dead=%v", suspected, dead)
	}
	if len(dead) != 0 {
		t.Fatalf("expected no DEAD peers yet, got %v", dead)
	}

	clock.advance(7 * time.Second) // totale 11s dall'ultimo contatto > timeoutDead
	suspected, dead = v.Sweep()
	if len(suspected) != 0 {
		t.Fatalf("expected no new SUSPECTED transitions, got %v", suspected)
	}
	if len(dead) != 1 || dead[0] != "peer-a:7000" {
		t.Fatalf("expected peer-a to become DEAD, got %v", dead)
	}
}

func TestRecordContactResetsTimeoutAndRevivesFromSuspected(t *testing.T) {
	v, clock := newViewWithFakeClock(3*time.Second, 10*time.Second)
	v.Seed([]string{"peer-a:7000"})

	clock.advance(4 * time.Second)
	v.Sweep() // peer-a diventa SUSPECTED

	v.RecordContact("peer-a:7000")

	clock.advance(1 * time.Second)
	suspected, dead := v.Sweep()
	if len(suspected) != 0 || len(dead) != 0 {
		t.Fatalf("expected peer-a to remain ALIVE after fresh contact, got suspected=%v dead=%v", suspected, dead)
	}
}

func TestDeadPeerStaysDeadUntilNewContact(t *testing.T) {
	v, clock := newViewWithFakeClock(3*time.Second, 10*time.Second)
	v.Seed([]string{"peer-a:7000"})

	clock.advance(11 * time.Second)
	_, dead := v.Sweep()
	if len(dead) != 1 {
		t.Fatalf("expected peer-a to be DEAD, got %v", dead)
	}

	// Un secondo sweep senza nuovi contatti non deve ri-segnalare il peer come DEAD.
	_, dead = v.Sweep()
	if len(dead) != 0 {
		t.Fatalf("expected no repeated DEAD notification, got %v", dead)
	}

	// Un nuovo contatto lo tratta come un nuovo join (torna ALIVE).
	v.RecordContact("peer-a:7000")
	peers := v.Snapshot()
	if peers[0].State != StateAlive {
		t.Fatalf("expected peer-a to be treated as a fresh join, got state=%s", peers[0].State)
	}
}

func TestAlivePeersExcludesDead(t *testing.T) {
	v, clock := newViewWithFakeClock(3*time.Second, 10*time.Second)
	v.Seed([]string{"peer-a:7000", "peer-b:7001"})

	clock.advance(11 * time.Second)
	v.Sweep() // entrambi diventano DEAD

	v.RecordContact("peer-b:7001") // solo peer-b torna ALIVE

	alive := v.AlivePeers()
	if len(alive) != 1 || alive[0] != "peer-b:7001" {
		t.Fatalf("expected only peer-b to be considered alive, got %v", alive)
	}
}
