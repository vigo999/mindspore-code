package configs

import "testing"

func TestDefaultConfigSetsTrainSyncTuningDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Training.RsyncCompress {
		t.Fatalf("expected rsync compression to be disabled by default")
	}
	if !cfg.Training.RsyncRespectGitIgnore {
		t.Fatalf("expected /train sync to respect .gitignore by default")
	}
	if cfg.Training.SyncParallelism != 0 {
		t.Fatalf("expected sync_parallelism default to use auto mode, got %d", cfg.Training.SyncParallelism)
	}
	if len(cfg.Training.Exclude) == 0 {
		t.Fatalf("expected built-in train excludes")
	}
}

func TestConfigValidateRejectsNegativeTrainSyncParallelism(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Training.Enabled = true
	cfg.Training.SyncParallelism = -1
	cfg.Training.HostsFile = ""
	cfg.Training.Hosts = []TrainingHostConfig{
		{Name: "gpuA", User: "user", Address: "gpu-a.example.com"},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validate to reject negative sync_parallelism")
	}
}
