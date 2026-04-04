package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/cloudnest/cloudnest-agent/internal/reporter"
	"github.com/cloudnest/cloudnest-agent/internal/storage"
)

type registerResponse struct {
	UUID  string `json:"uuid"`
	Token string `json:"token"`
}

func RegisterWithMaster(cfg *Config, regToken string) error {
	if err := storage.EnsureDataDirs(); err != nil {
		return fmt.Errorf("failed to initialize data directory: %w", err)
	}
	if len(cfg.ScanDirs) == 0 {
		cfg.ScanDirs = []string{storage.FilesDir()}
	}

	hostname, _ := os.Hostname()

	diskTotal, _ := reporter.GetDiskTotal()

	body := map[string]interface{}{
		"hostname":  hostname,
		"ip":        "", // Will be detected by master from request
		"port":      cfg.Port,
		"region":    "",
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
		"cpu_model": reporter.GetCPUModel(),
		"cpu_cores": runtime.NumCPU(),
		"disk_total": diskTotal,
		"ram_total":  reporter.GetRAMTotal(),
		"version":   "0.1.0",
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", cfg.MasterURL+"/api/agent/register", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+regToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to master: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registration failed with status %d", resp.StatusCode)
	}

	var result registerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	cfg.UUID = result.UUID
	cfg.Token = result.Token

	return SaveConfig(cfg)
}
