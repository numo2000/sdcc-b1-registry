// Package config gestisce il caricamento della configurazione del nodo
// (porta, seed list, intervallo di gossip, timeout di failure detection) da
// file esterno, senza valori hard-coded.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config e' la configurazione di un nodo del registry, cosi' come descritta
// in configs/config.example.yaml.
type Config struct {
	Node struct {
		ID         string `yaml:"id"`
		ListenPort int    `yaml:"listen_port"`
		GRPCPort   int    `yaml:"grpc_port"`
	} `yaml:"node"`

	Bootstrap struct {
		Seeds []string `yaml:"seeds"`
	} `yaml:"bootstrap"`

	Gossip struct {
		RoundIntervalMs int `yaml:"round_interval_ms"`
		PeersPerRound   int `yaml:"peers_per_round"`

		// DeadCheckEveryNRounds, se > 0, contatta anche il DEAD meno
		// recente ogni tot round indipendentemente dal pool ALIVE:
		// mitigazione per la guarigione di una partizione di rete
		// (vedi gossip.Service.doRound). 0 disabilita il controllo.
		DeadCheckEveryNRounds int `yaml:"dead_check_every_n_rounds"`
	} `yaml:"gossip"`

	FailureDetection struct {
		TimeoutSuspectMs int `yaml:"timeout_suspect_ms"`
		TimeoutDeadMs    int `yaml:"timeout_dead_ms"`
	} `yaml:"failure_detection"`

	// ServiceHealth controlla il controllo periodico di raggiungibilita'
	// sugli endpoint dei servizi registrati (vedi
	// gossip.Service.doHealthCheck): gira su un ciclo separato dal round di
	// gossip, apposta per non aggiungere latenza di rete al percorso di
	// lettura (Discover resta una lookup locale, non contatta mai il
	// servizio a runtime).
	ServiceHealth struct {
		CheckIntervalMs int `yaml:"check_interval_ms"`
		CheckTimeoutMs  int `yaml:"check_timeout_ms"`
	} `yaml:"service_health"`
}

// Load legge e valida la configurazione dal file YAML indicato da path.
func Load(path string) (Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("config: impossibile leggere %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("config: impossibile interpretare %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return cfg, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.Node.ID == "" {
		return fmt.Errorf("node.id e' obbligatorio")
	}
	if c.Node.ListenPort <= 0 {
		return fmt.Errorf("node.listen_port deve essere positivo")
	}
	if c.Node.GRPCPort <= 0 {
		return fmt.Errorf("node.grpc_port deve essere positivo")
	}
	if c.Gossip.RoundIntervalMs <= 0 {
		return fmt.Errorf("gossip.round_interval_ms deve essere positivo")
	}
	if c.Gossip.PeersPerRound <= 0 {
		return fmt.Errorf("gossip.peers_per_round deve essere positivo")
	}
	if c.Gossip.DeadCheckEveryNRounds < 0 {
		return fmt.Errorf("gossip.dead_check_every_n_rounds non puo' essere negativo (0 = disabilitato)")
	}
	if c.FailureDetection.TimeoutSuspectMs <= 0 || c.FailureDetection.TimeoutDeadMs <= 0 {
		return fmt.Errorf("i timeout di failure detection devono essere positivi")
	}
	if c.FailureDetection.TimeoutDeadMs <= c.FailureDetection.TimeoutSuspectMs {
		return fmt.Errorf("timeout_dead_ms deve essere maggiore di timeout_suspect_ms")
	}
	if c.ServiceHealth.CheckIntervalMs < 0 {
		return fmt.Errorf("service_health.check_interval_ms non puo' essere negativo (0 = disabilitato)")
	}
	if c.ServiceHealth.CheckTimeoutMs <= 0 {
		return fmt.Errorf("service_health.check_timeout_ms deve essere positivo")
	}
	return nil
}

// I tre metodi seguenti convertono i campi *Ms (interi grezzi in
// millisecondi) nel time.Duration richiesto dalle API di gossip/membership.

// GossipRoundInterval e' l'intervallo tra un round di gossip e il successivo.
func (c Config) GossipRoundInterval() time.Duration {
	return time.Duration(c.Gossip.RoundIntervalMs) * time.Millisecond
}

// TimeoutSuspect e' il tempo senza contatto prima di marcare un peer SUSPECTED.
func (c Config) TimeoutSuspect() time.Duration {
	return time.Duration(c.FailureDetection.TimeoutSuspectMs) * time.Millisecond
}

// TimeoutDead e' il tempo senza contatto prima di marcare un peer DEAD.
func (c Config) TimeoutDead() time.Duration {
	return time.Duration(c.FailureDetection.TimeoutDeadMs) * time.Millisecond
}

// ServiceHealthCheckInterval e' l'intervallo tra un controllo di
// raggiungibilita' sui servizi registrati e il successivo (0 disabilita).
func (c Config) ServiceHealthCheckInterval() time.Duration {
	return time.Duration(c.ServiceHealth.CheckIntervalMs) * time.Millisecond
}

// ServiceCheckTimeout e' il timeout di connessione per un singolo controllo.
func (c Config) ServiceCheckTimeout() time.Duration {
	return time.Duration(c.ServiceHealth.CheckTimeoutMs) * time.Millisecond
}
