package ansible

import "github.com/etkecc/go-kit"

func MergeHost(base, add *Host) *Host {
	if base == nil && add == nil {
		return nil
	}

	if base == nil {
		return add
	}

	if add == nil {
		return base
	}

	if base.Name == "" {
		base.Name = add.Name
	}
	if base.Host == "" {
		base.Host = add.Host
	}
	if base.Port == 0 {
		base.Port = add.Port
	}
	if base.User == "" {
		base.User = add.User
	}
	if base.SSHPass == "" {
		base.SSHPass = add.SSHPass
	}
	if base.BecomePass == "" {
		base.BecomePass = add.BecomePass
	}
	if len(base.PrivateKeys) == 0 {
		base.PrivateKeys = add.PrivateKeys
	}
	base.Groups = kit.Uniq(append(base.Groups, add.Groups...))
	return base
}
