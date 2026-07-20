package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

func TestLoadValidConfig(t *testing.T) {
	path := writeTempConfig(t, `
node:
  id: "node-1"
  listen_port: 7000
bootstrap:
  seeds:
    - "node-2:7000"
gossip:
  round_interval_ms: 1000
  peers_per_round: 1
failure_detection:
  timeout_suspect_ms: 3000
  timeout_dead_ms: 10000
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Node.ID != "node-1" || cfg.Node.ListenPort != 7000 {
		t.Errorf("unexpected node config: %+v", cfg.Node)
	}
	if len(cfg.Bootstrap.Seeds) != 1 || cfg.Bootstrap.Seeds[0] != "node-2:7000" {
		t.Errorf("unexpected seeds: %+v", cfg.Bootstrap.Seeds)
	}
	if cfg.GossipRoundInterval().Milliseconds() != 1000 {
		t.Errorf("unexpected gossip round interval: %v", cfg.GossipRoundInterval())
	}
	if cfg.TimeoutSuspect().Milliseconds() != 3000 || cfg.TimeoutDead().Milliseconds() != 10000 {
		t.Errorf("unexpected timeouts: suspect=%v dead=%v", cfg.TimeoutSuspect(), cfg.TimeoutDead())
	}
}

func TestLoadRejectsMissingNodeID(t *testing.T) {
	path := writeTempConfig(t, `
node:
  listen_port: 7000
gossip:
  round_interval_ms: 1000
  peers_per_round: 1
failure_detection:
  timeout_suspect_ms: 3000
  timeout_dead_ms: 10000
`)

	if _, err := Load(path); err == nil {
		t.Errorf("expected error for missing node.id")
	}
}

func TestLoadRejectsDeadTimeoutNotGreaterThanSuspect(t *testing.T) {
	path := writeTempConfig(t, `
node:
  id: "node-1"
  listen_port: 7000
gossip:
  round_interval_ms: 1000
  peers_per_round: 1
failure_detection:
  timeout_suspect_ms: 5000
  timeout_dead_ms: 3000
`)

	if _, err := Load(path); err == nil {
		t.Errorf("expected error when timeout_dead_ms <= timeout_suspect_ms")
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Errorf("expected error for missing config file")
	}
}
