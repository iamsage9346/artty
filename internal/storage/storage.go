package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type LastSession struct {
	Profile    string `json:"profile"`
	Region     string `json:"region"`
	InstanceID string `json:"instance_id"`
	RDSHost    string `json:"rds_host"`
	RDSPort    int    `json:"rds_port"`
	LocalPort  int    `json:"local_port"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".artty", "session.json"), nil
}

func Save(s LastSession) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func Load() (LastSession, bool) {
	path, err := configPath()
	if err != nil {
		return LastSession{}, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return LastSession{}, false
	}
	var s LastSession
	if err := json.Unmarshal(b, &s); err != nil {
		return LastSession{}, false
	}
	return s, true
}
