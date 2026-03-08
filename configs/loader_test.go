package configs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFileLoadsTrainingHostsFile(t *testing.T) {
	tmpDir := t.TempDir()

	configPath := filepath.Join(tmpDir, "mscli.yaml")
	hostsPath := filepath.Join(tmpDir, "train_hosts.yaml")

	configBody := []byte(`
training:
  enabled: true
  local_path: .
  startup_command: source ~/.bashrc && conda activate trainer
  remote_code_path: ~/workspace/demo
  run_base_dir: ~/workspace/runs
  train_command: python -u examples/fake_log_generator.py --run-id {{RUN_ID}} --host {{HOST_NAME}}
  hosts_file: train_hosts.yaml
`)
	hostsBody := []byte(`
hosts:
  - name: gpuA
    user: alice
    address: gpu-a.example.com
    startup_command: source ~/.bashrc && conda activate gpu-a
  - name: gpuB
    user: bob
    address: gpu-b.example.com
`)

	if err := os.WriteFile(configPath, configBody, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(hostsPath, hostsBody, 0600); err != nil {
		t.Fatalf("write hosts: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile returned error: %v", err)
	}

	if len(cfg.Training.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(cfg.Training.Hosts))
	}
	if cfg.Training.Hosts[0].Name != "gpuA" || cfg.Training.Hosts[1].Name != "gpuB" {
		t.Fatalf("unexpected hosts: %+v", cfg.Training.Hosts)
	}
	if cfg.Training.StartupCommand != "source ~/.bashrc && conda activate trainer" {
		t.Fatalf("unexpected global startup command: %q", cfg.Training.StartupCommand)
	}
	if cfg.Training.Hosts[0].StartupCommand != "source ~/.bashrc && conda activate gpu-a" {
		t.Fatalf("unexpected host startup command: %q", cfg.Training.Hosts[0].StartupCommand)
	}
	if cfg.Training.HostsFile != hostsPath {
		t.Fatalf("expected resolved hosts file path %q, got %q", hostsPath, cfg.Training.HostsFile)
	}
}
