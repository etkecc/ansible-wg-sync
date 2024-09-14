package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/etkecc/ansible-wg-sync/internal/config"
)

func handleWireGuard(cfg *config.Config, allowedIPs, postUp, postDown []string) error {
	if cfg.ProfilePath == "" {
		return nil
	}

	logger.Println("updating WireGuard profle", cfg.ProfilePath)
	name := strings.Replace(filepath.Base(cfg.ProfilePath), filepath.Ext(cfg.ProfilePath), "", 1)
	if err := updateWGProfile(name, allowedIPs, postUp, postDown, cfg.ProfilePath, cfg.Table); err != nil {
		return err
	}

	logger.Println("reloading WireGuard interface", name)
	if err := exec.Command("wg", "show", name).Run(); err != nil { // if interface does not exist, start it
		return exec.Command("systemctl", "start", "wg-quick@"+name).Run() //nolint:gosec // that's ok
	}
	// if interface exists, restart it
	return exec.Command("systemctl", "restart", "wg-quick@"+name).Run() //nolint:gosec // that's ok
}

func updateWGProfile(name string, allowedIPs, postUp, postDown []string, path string, table int) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(contents), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "Table") && table > 0 {
			lines[i] = "Table = " + strconv.Itoa(table)
		}
		if strings.HasPrefix(line, "AllowedIPs") {
			lines[i] = "AllowedIPs = " + strings.Join(allowedIPs, ",")
		}
		if strings.HasPrefix(line, "PostUp") && len(postUp) > 0 {
			lines[i] = "PostUp = " + strings.Join(postUp, "; ")
		}
		if strings.HasPrefix(line, "PostDown") && len(postDown) > 0 {
			lines[i] = "PostDown = " + strings.Join(postDown, "; ")
		}
	}

	contents, err = applyVars(strings.Join(lines, "\n"), map[string]any{"name": name, "table": table})
	if err != nil {
		return err
	}

	return os.WriteFile(path, contents, 0o600)
}

func applyVars(tplString string, vars map[string]any) ([]byte, error) {
	var result bytes.Buffer
	tpl, err := template.New("template").Parse(tplString)
	if err != nil {
		return nil, err
	}
	err = tpl.Execute(&result, vars)
	if err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}
