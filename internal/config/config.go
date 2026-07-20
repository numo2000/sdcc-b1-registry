// Package config gestisce il caricamento della configurazione del nodo
// (porta, seed list, intervallo di gossip, timeout di failure detection) da
// file esterno, senza valori hard-coded.
//
// Vedi ARCHITETTURA.md, sezione 10.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config e' la configurazione di un nodo del registry, cosi' come descritta
// in ARCHITETTURA.md sezione 10 e configs/config.example.yaml.
type Config struct {
	Node struct {
		ID         string `yaml:"id"`
		ListenPort int    `yaml:"listen_port"`
	} `yaml:"node"`

	Bootstrap struct {
		Seeds []string `yaml:"seeds"`
	} `yaml:"bootstrap"`

	Gossip struct {
		RoundIntervalMs int `yaml:"round_interval_ms"`
		PeersPerRound   int `yaml:"peers_per_round"`
	} `yaml:"gossip"`

	FailureDetection struct {
		TimeoutSuspectMs int `yaml:"timeout_suspect_ms"`
		TimeoutDeadMs    int `yaml:"timeout_dead_ms"`
	} `yaml:"failure_detection"`
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
	if c.Gossip.RoundIntervalMs <= 0 {
		return fmt.Errorf("gossip.round_interval_ms deve essere positivo")
	}
	if c.Gossip.PeersPerRound <= 0 {
		return fmt.Errorf("gossip.peers_per_round deve essere positivo")
	}
	if c.FailureDetection.TimeoutSuspectMs <= 0 || c.FailureDetection.TimeoutDeadMs <= 0 {
		return fmt.Errorf("i timeout di failure detection devono essere positivi")
	}
	if c.FailureDetection.TimeoutDeadMs <= c.FailureDetection.TimeoutSuspectMs {
		return fmt.Errorf("timeout_dead_ms deve essere maggiore di timeout_suspect_ms")
	}
	return nil
}

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
