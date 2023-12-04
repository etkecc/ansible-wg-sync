package ansible

import (
	"bufio"
	"bytes"
	"os"
	"strings"
)

const defaultSection = "unknown"

type AnsibleCfg struct {
	Config map[string]map[string]string
}

func NewAnsibleCfgFile(f string) (*AnsibleCfg, error) {
	if f == "" {
		return nil, nil
	}

	if !FileExists(f) {
		return nil, os.ErrNotExist
	}

	bs, err := os.ReadFile(f)
	if err != nil {
		return &AnsibleCfg{Config: make(map[string]map[string]string)}, err
	}

	return NewAnsibleCfgParser(bs), nil
}

func NewAnsibleCfgParser(input []byte) *AnsibleCfg {
	cfg := &AnsibleCfg{}
	cfg.parse(input)
	return cfg
}

func (a *AnsibleCfg) parse(input []byte) {
	a.Config = make(map[string]map[string]string)

	activeSectionName := defaultSection
	scanner := bufio.NewScanner(bytes.NewBuffer(input))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch parseType(line) { //nolint:exhaustive // intended
		case TypeGroup:
			activeSectionName = parseGroup(line)
		case TypeVar:
			key, value := parseVar(line)
			if _, ok := a.Config[activeSectionName]; !ok {
				a.Config[activeSectionName] = map[string]string{}
			}
			a.Config[activeSectionName][key] = value
		}
	}
}
