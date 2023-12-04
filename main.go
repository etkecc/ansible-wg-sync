package main

import (
	"log"
	"net"
	"os"
	"regexp"

	"github.com/adrg/xdg"

	"gitlab.com/etke.cc/go/ansible"
	"gitlab.com/etke.cc/int/ansible-wg-sync/config"
)

var (
	withDebug   bool
	logger      = log.New(os.Stdout, "[ansible-wg-sync] ", 0)
	domainRegex = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9]$`)
)

func main() {
	path, err := xdg.SearchConfigFile("ansible-wg-sync.yml")
	if err != nil {
		logger.Fatal("cannot find the ansible-wg-sync.yml config file: ", err, ", ensure it is in $XDG_CONFIG_DIRS or $XDG_CONFIG_HOME of the root(!) user")
	}
	if !isRoot() {
		logger.Println("WARNING: not running as root, profile updates will fail")
	}

	cfg, err := config.Read(path)
	if err != nil {
		logger.Fatal("cannot read the ", path, " config file:", err)
	}
	withDebug = cfg.Debug
	allowedIPs := getAllowedIPs(cfg)
	logger.Println("loaded", len(allowedIPs), "allowed IPs")
	if len(allowedIPs) == 0 {
		logger.Println("WARNING: no allowed IPs found")
		return
	}
	if err := handleWireGuard(cfg, allowedIPs, cfg.PostUp, cfg.PostDown); err != nil {
		logger.Println("ERROR: cannot update WireGuard profile:", err)
	}
	if err := handleNetworkManager(cfg, allowedIPs); err != nil {
		logger.Println("ERROR: cannot update NetworkManager profile:", err)
	}
}

func parseCIDR(host string) string {
	// if CIDR, return as is
	if _, _, err := net.ParseCIDR(host); err == nil {
		return host
	}
	// if IP, return CIDR
	if ip := net.ParseIP(host); ip != nil {
		return ip.String() + "/32"
	}
	// check if domain
	if len(host) < 4 || len(host) > 77 {
		return ""
	}
	if !domainRegex.MatchString(host) {
		return ""
	}

	// if domain with A record, return CIDR
	if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
		return ips[0].String() + "/32"
	}
	// if domain with CNAME record, run again
	if cname, err := net.LookupCNAME(host); err == nil && cname != "" {
		return parseCIDR(cname)
	}

	return ""
}

func getAllowedIPs(cfg *config.Config) []string {
	allowedIPs := []string{}
	for _, ip := range cfg.AllowedIPs {
		cidr := parseCIDR(ip)
		if cidr == "" {
			debug("allowed IP", ip, "is not an IP address")
			continue
		}
		allowedIPs = append(allowedIPs, cidr)
	}

	for _, invPath := range cfg.InventoryPaths {
		inv, err := ansible.NewHostsFile(invPath, &ansible.Host{})
		if err != nil {
			logger.Println("ERROR: cannot read inventory file", invPath, ":", err)
			continue
		}
		if inv == nil || len(inv.Hosts) == 0 {
			debug("inventory", invPath, "is empty")
			continue
		}
		for _, host := range inv.Hosts {
			cidr := parseCIDR(host.Host)
			if cidr == "" {
				debug("host", host.Host, "is not an IP address")
				continue
			}
			allowedIPs = append(allowedIPs, cidr)
		}
	}
	return ansible.Uniq(allowedIPs)
}

func isRoot() bool {
	return os.Geteuid() == 0
}

func debug(args ...any) {
	if !withDebug {
		return
	}
	logger.Println(args...)
}
