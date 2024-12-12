package ansible

import (
	"errors"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/etkecc/go-kit"
)

// ParseInventory using ansible.cfg and hosts (ini) files
func ParseInventory(ansibleCfg, hostsini, limit string) *Inventory {
	if ansibleCfg == "" {
		ansibleCfg = path.Join(path.Dir(path.Dir(hostsini)), "/ansible.cfg")
	}

	acfg, err := NewAnsibleCfgFile(ansibleCfg)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Println("cannot parse", ansibleCfg, "error:", err)
		return nil
	}

	paths := parseAllInventoryPaths(hostsini, acfg)
	defaults := parseDefaultsFromAnsibleCfg(acfg)
	inv := parseHostsFiles(paths, parseLimit(limit), defaults)
	if inv == nil {
		return nil
	}

	var wg sync.WaitGroup
	for name := range inv.Hosts {
		wg.Add(1)
		go func(name string, wg *sync.WaitGroup) {
			defer wg.Done()
			inv.Hosts[name].Dirs, inv.Hosts[name].Files = parseAdditionalFiles(paths, name)

			vars := parseHostVars(paths, name)
			if vars == nil {
				return
			}
			inv.Hosts[name].Vars = vars
			if sshPrivateKey := inv.Hosts[name].Vars.String("ansible_ssh_private_key_file"); sshPrivateKey != "" {
				inv.Hosts[name].PrivateKeys = kit.Uniq(append(inv.Hosts[name].PrivateKeys, sshPrivateKey))
			}
		}(name, &wg)
	}
	wg.Wait()
	return inv
}

func parseHostsFiles(paths, only []string, defaults *Host) *Inventory {
	inv := &Inventory{}
	for _, path := range paths {
		parsedInv, err := NewHostsFile(path, defaults, only...)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Println("cannot parse", path, "error:", err)
			continue
		}
		if parsedInv != nil {
			parsedInv.Paths = []string{path}
		}

		inv.Merge(parsedInv)
	}

	if len(inv.Hosts) == 0 {
		return nil
	}

	return inv
}

func parseHostVars(hostsPaths []string, name string) HostVars {
	allvars := []HostVars{}
	for _, hostsPath := range hostsPaths {
		varsPath := path.Join(path.Dir(hostsPath), "/host_vars/", name, "/vars.yml")
		vars, err := NewHostVarsFile(varsPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Println("cannot parse", varsPath, "error:", err)
			continue
		}
		allvars = append(allvars, vars)
	}
	if len(allvars) == 0 {
		return nil
	}

	final := HostVars{}
	for _, vars := range allvars {
		for k, v := range vars {
			if _, ok := final[k]; ok {
				continue
			}
			final[k] = v
		}
	}
	return final
}

// parseAdditionalFiles returns list of dirs to create, map of files (source => target) and error
func parseAdditionalFiles(invPaths []string, name string) (dirs []string, files map[string]string) {
	hostvarsDir := path.Join("/host_vars", name)
	groupvarsDir := "/group_vars"

	dirs = []string{hostvarsDir, groupvarsDir}
	files = map[string]string{}

	for _, invPath := range invPaths {
		hostvarsPath := path.Join(path.Dir(invPath), hostvarsDir)
		groupvarsPath := path.Join(path.Dir(invPath), groupvarsDir)
		pDirs, pFiles, err := findFilesAndDirs(hostvarsPath, hostvarsDir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Println("cannot parse", hostvarsPath, "error:", err)
			continue
		}
		dirs = append(dirs, pDirs...)
		for k, v := range pFiles {
			files[k] = v
		}
		gDirs, gFiles, err := findFilesAndDirs(groupvarsPath, groupvarsDir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Println("cannot parse", groupvarsPath, "error:", err)
			continue
		}

		dirs = append(dirs, gDirs...)
		for k, v := range gFiles {
			files[k] = v
		}

	}

	return kit.Uniq(dirs), files
}

func findFilesAndDirs(src, prepend string) (dirs []string, files map[string]string, err error) {
	dirs = []string{}
	files = map[string]string{}
	err = filepath.Walk(src, func(itempath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if itempath == src {
			return nil
		}
		name := path.Join(prepend, strings.Replace(itempath, src, "", 1))
		if info.IsDir() {
			dirs = append(dirs, name)
			return nil
		}

		files[itempath] = name
		return nil
	})
	return dirs, files, err
}
