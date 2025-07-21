package sardis

import (
	"context"
	"fmt"
	"slices"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/jasper"
)

type NetworkConf struct {
	Hosts []HostDefinition `bson:"hosts" json:"hosts" yaml:"hosts"`
}

type HostDefinition struct {
	Name     string `bson:"name" json:"name" yaml:"name"`
	User     string `bson:"user" json:"user" yaml:"user"`
	Hostname string `bson:"host" json:"host" yaml:"host"`
	Port     int    `bson:"port" json:"port" yaml:"port"`
	Protocol string `bson:"protocol" json:"protocol" yaml:"protocol"`
	Sardis   bool   `bson:"has_sardis" json:"has_sardis" yaml:"has_sardis"`
}

func (n *NetworkConf) Validate() error {
	ec := &erc.Collector{}

	for idx := range n.Hosts {
		ec.Wrapf(n.Hosts[idx].Validate(), "%d of %T is not valid", idx, n.Hosts[idx])
	}

	return ec.Resolve()
}

func (n *NetworkConf) ByName(name string) (*HostDefinition, error) {
	for _, h := range n.Hosts {
		if h.Name == name {
			return &h, nil
		}
	}

	return nil, fmt.Errorf("could not find a host named '%s'", name)
}

func (h *HostDefinition) Validate() error {
	ec := &erc.Collector{}

	ec.When(h.Name == "", ers.Error("cannot have an empty name for a host"))
	ec.When(h.Hostname == "", ers.Error("cannot have an empty host name"))
	ec.When(h.Port == 0, ers.Error("must specify a non-zero port number for a host"))
	ec.When(!slices.Contains([]string{"ssh", "jasper"}, h.Protocol), ers.Error("host protocol must be ssh or jasper"))

	if h.Protocol == "ssh" {
		ec.When(h.User == "", ers.Error("must specify user for ssh hosts"))
	}

	return ec.Resolve()
}

func (h *HostDefinition) Jasper(ctx context.Context) (jasper.Manager, error) {
	// switch h.Protocol {
	// case "ssh":
	// 	remoteOpts := options.Remote{
	// 		RemoteConfig: options.RemoteConfig{
	// 			Host: h.Hostname,
	// 			User: h.User,
	// 		}}
	// 	clientOpts := jaspercli.ClientOptions{
	// 		BinaryPath: "sardis",
	// 		Type:       jaspercli.RPCService,
	// 		Port:       h.Port,
	// 	}

	// 	return jaspercli.NewSSHClient(remoteOpts, clientOpts, true)
	// case "jasper":
	// 	addrStr := fmt.Sprintf("%s:%d", h.Hostname, h.Port)

	// 	serviceAddr, err := net.ResolveTCPAddr("tcp", addrStr)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("could not resolve Jasper service address at '%s': %w", addrStr, err)
	// 	}

	// 	dialCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	// 	defer cancel()

	// 	return remote.NewRPCClient(dialCtx, serviceAddr, nil)
	// default:
	// 	return nil, errors.New("unsupported jasper protocol")
	// }
	panic("unsupported")
}
