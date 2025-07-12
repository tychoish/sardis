package sardis

import (
	"context"

	"github.com/tychoish/jasper"
)

type HostConf struct {
	Name     string `bson:"name" json:"name" yaml:"name"`
	User     string `bson:"user" json:"user" yaml:"user"`
	Hostname string `bson:"host" json:"host" yaml:"host"`
	Port     int    `bson:"port" json:"port" yaml:"port"`
	Protocol string `bson:"protocol" json:"protocol" yaml:"protocol"`
	Sardis   bool   `bson:"has_sardis" json:"has_sardis" yaml:"has_sardis"`
}

func (h *HostConf) Jasper(ctx context.Context) (jasper.Manager, error) {
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
