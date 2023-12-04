package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gitlab.com/etke.cc/int/ansible-wg-sync/config"
)

func handleNetworkManager(cfg *config.Config, allowedIPs []string) error {
	if cfg.NMProfilePath == "" {
		return nil
	}

	logger.Println("updating NetworkManager profile", cfg.NMProfilePath)
	if err := updateNMProfile(allowedIPs, cfg.NMProfilePath); err != nil {
		return err
	}

	logger.Println("reloading NetworkManager")
	if err := exec.Command("nmcli", "connection", "reload").Run(); err != nil {
		return err
	}

	logger.Println("restarting NetworkManager connection")
	if err := exec.Command("nmcli", "connection", "down", cfg.NMProfilePath).Run(); err != nil { //nolint:gosec // that's ok
		logger.Println("failed to stop NetworkManager connection:", err)
	}

	return exec.Command("nmcli", "connection", "up", cfg.NMProfilePath).Run() //nolint:gosec // that's ok
}

func updateNMProfile(allowedIPs []string, path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(contents), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "allowed-ips=") {
			lines[i] = "allowed-ips=" + strings.Join(allowedIPs, ";") + ";" // add trailing semicolon
			break
		}
	}
	contents = []byte(strings.Join(lines, "\n"))
	return os.WriteFile(path, contents, 0o600)
}

func handleWireGuard(cfg *config.Config, allowedIPs, postUp, postDown []string) error {
	if cfg.WGProfilePath == "" {
		return nil
	}

	logger.Println("updating WireGuard profle", cfg.WGProfilePath)
	if err := updateWGProfile(allowedIPs, postUp, postDown, cfg.WGProfilePath, cfg.Table); err != nil {
		return err
	}

	name := strings.Replace(filepath.Base(cfg.WGProfilePath), filepath.Ext(cfg.WGProfilePath), "", 1)
	logger.Println("reloading WireGuard interface", name)
	return exec.Command("systemctl", "restart", "wg-quick@"+name).Run() //nolint:gosec // that's ok
}

func updateWGProfile(allowedIPs, postUp, postDown []string, path string, table int) error {
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
	contents = []byte(strings.Join(lines, "\n"))
	return os.WriteFile(path, contents, 0o600)
}
