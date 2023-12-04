package ansible

import (
	"bufio"
	"bytes"
	"os"
	"path"
	"strings"

	"golang.org/x/exp/slices"
)

const (
	defaultGroup = "ungrouped"
	todo         = "todo"
)

// Inventory contains all hosts file content
type Inventory struct {
	cacheGroupVars map[string]*Host    // cached calculated group vars
	cacheGroups    map[string][]string // cached calculated group tree
	only           []string            // limit parsing only to the following hosts

	Groups    map[string][]*Host           // host-by-group
	GroupVars map[string]map[string]string // group vars
	GroupTree map[string][]string          // raw group tree
	Hosts     map[string]*Host             // hosts-by-name
}

// Host is a parsed host
type Host struct {
	Vars       HostVars // host vars
	Dirs       []string
	Files      map[string]string
	Group      string   // main group
	Groups     []string // all related groups
	Name       string   // host name
	Host       string   // host address
	Port       int      // host port
	User       string   // host user
	SSHPass    string   // host ssh password
	BecomePass string   // host become password
	PrivateKey string   // host ssh private key
}

func (h *Host) FindFile(name string) (string, bool) {
	if h == nil || len(h.Dirs) == 0 || len(h.Files) == 0 {
		return "", false
	}

	for _, dir := range h.Dirs {
		fullpath := path.Join(dir, name)
		for k, v := range h.Files {
			if v == fullpath {
				return k, true
			}
		}
	}
	return "", false
}

// HasTODOs returns true if host has any todo values
func (h *Host) HasTODOs() bool {
	if h == nil {
		return true
	}
	if strings.ToLower(h.Host) == todo {
		return true
	}
	if strings.ToLower(h.User) == todo {
		return true
	}
	if strings.ToLower(h.SSHPass) == todo {
		return true
	}
	if strings.ToLower(h.BecomePass) == todo {
		return true
	}
	if strings.ToLower(h.PrivateKey) == todo {
		return true
	}
	if strings.ToLower(h.Name) == todo {
		return true
	}

	return h.Vars.HasTODOs()
}

func NewHostsFile(f string, defaults *Host, only ...string) (*Inventory, error) {
	if !FileExists(f) {
		return nil, os.ErrNotExist
	}

	bs, err := os.ReadFile(f)
	if err != nil {
		return &Inventory{}, err
	}

	return NewHostsParser(bs, defaults, only...), nil
}

func NewHostsParser(input []byte, defaults *Host, only ...string) *Inventory {
	hosts := &Inventory{only: only}
	hosts.init()
	hosts.parse(input, defaults)
	return hosts
}

func (i *Inventory) init() {
	i.Groups = make(map[string][]*Host)
	i.Groups[defaultGroup] = make([]*Host, 0)

	i.GroupTree = make(map[string][]string)
	i.GroupTree[defaultGroup] = make([]string, 0)

	i.GroupVars = make(map[string]map[string]string)
	i.GroupVars[defaultGroup] = make(map[string]string)

	i.Hosts = make(map[string]*Host)

	i.cacheGroupVars = make(map[string]*Host)
	i.cacheGroups = make(map[string][]string)
}

// findAllGroups tries to find all groups related to the group. Experimental
func (i *Inventory) findAllGroups(groups []string) []string {
	cachekey := strings.Join(groups, ",")
	cached := i.cacheGroups[cachekey]
	if cached != nil {
		return cached
	}

	all := groups
	for name, children := range i.GroupTree {
		if slices.Contains(groups, name) {
			all = append(all, name)
			all = append(all, children...)
			continue
		}
		for _, child := range children {
			if slices.Contains(groups, child) {
				all = append(all, name)
				break
			}
		}
	}
	all = Uniq(all)
	if strings.Join(all, ",") != cachekey {
		all = i.findAllGroups(all)
	}
	i.cacheGroups[cachekey] = all

	return all
}

func (i *Inventory) initGroup(name string) {
	if _, ok := i.Groups[name]; !ok {
		i.Groups[name] = make([]*Host, 0)
	}
	if _, ok := i.GroupTree[name]; !ok {
		i.GroupTree[name] = make([]string, 0)
	}
	if _, ok := i.GroupVars[name]; !ok {
		i.GroupVars[name] = make(map[string]string)
	}
}

func (i *Inventory) parse(input []byte, defaults *Host) {
	activeGroupName := defaultGroup
	buff := bytes.NewBuffer(input)
	scanner := bufio.NewScanner(buff)
	for scanner.Scan() {
		line := scanner.Text()
		switch parseType(line) { //nolint:exhaustive // intended
		case TypeGroup:
			activeGroupName = parseGroup(line)
			i.initGroup(activeGroupName)
		case TypeGroupVars:
			activeGroupName = parseGroup(line)
			i.initGroup(activeGroupName)
		case TypeGroupChildren:
			activeGroupName = parseGroup(line)
			i.initGroup(activeGroupName)
		case TypeGroupChild:
			group := parseGroup(line)
			i.initGroup(group)
			i.GroupTree[activeGroupName] = append(i.GroupTree[activeGroupName], group)
		case TypeHost:
			host := parseHost(line, i.only)
			if host != nil {
				host.Group = activeGroupName
				host.Groups = []string{activeGroupName}
				i.Hosts[host.Name] = host
			}
		case TypeVar:
			k, v := parseVar(line)
			i.GroupVars[activeGroupName][k] = v
		}
	}
	i.finalize(defaults)
}

// groupParams converts group vars map[key]value into []string{"key=value"}
func (i *Inventory) groupParams(group string) []string {
	vars := i.GroupVars[group]
	if len(vars) == 0 {
		return nil
	}

	params := make([]string, 0, len(vars))
	for k, v := range vars {
		params = append(params, k+"="+v)
	}
	return params
}

// getGroupVars returns merged group vars. Experimental
func (i *Inventory) getGroupVars(groups []string) *Host {
	cachekey := strings.Join(Uniq(groups), ",")
	cached := i.cacheGroupVars[cachekey]
	if cached != nil {
		return cached
	}

	vars := &Host{}
	for _, group := range groups {
		groupVars := parseParams(i.groupParams(group))
		if groupVars == nil {
			continue
		}
		vars = MergeHost(vars, parseParams(i.groupParams(group)))
	}

	i.cacheGroupVars[cachekey] = vars
	return vars
}

func (i *Inventory) finalize(defaults *Host) {
	for _, host := range i.Hosts {
		host.Groups = i.findAllGroups(Uniq(host.Groups))
		host = MergeHost(host, i.getGroupVars(host.Groups))
		host = MergeHost(host, defaults)
		i.Hosts[host.Name] = host

		for _, group := range host.Groups {
			i.Groups[group] = append(i.Groups[group], host)
		}
	}
}

// Match a host by name
func (i *Inventory) Match(m string) *Host {
	return i.Hosts[m]
}

// Merge does append and replace
func (i *Inventory) Merge(h2 *Inventory) {
	if i.Groups == nil {
		i.Groups = make(map[string][]*Host)
	}
	if i.Hosts == nil {
		i.Hosts = make(map[string]*Host)
	}
	if h2 == nil {
		return
	}

	for group := range h2.Groups {
		if _, ok := i.Groups[group]; !ok {
			i.Groups[group] = make([]*Host, 0)
		}
	}

	for name, host := range h2.Hosts {
		i.Hosts[name] = MergeHost(i.Hosts[name], host)
	}
}
