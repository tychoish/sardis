package sardis

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/mongodb/jasper"
	jaspercli "github.com/mongodb/jasper/cli"
	"github.com/mongodb/jasper/rpc"
	"github.com/pkg/errors"
)

type HostConf struct {
	Name     string `bson:"name" json:"name" yaml:"name"`
	User     string `bson:"host" json:"host" yaml:"host"`
	Hostname string `bson:"host_name" json:"host_name" yaml:"host_name"`
	Port     int    `bson:"port" json:"port" yaml:"port"`
	Protocol string `bson:"protocol" json:"protocol" yaml:"protocol"`
}

func (h *HostConf) Jasper(ctx context.Context) (jasper.Manager, error) {
	switch h.Protocol {
	case "ssh":
		remoteOpts := jasper.RemoteOptions{
			Host: h.Hostname,
			User: h.User,
		}
		clientOpts := jaspercli.ClientOptions{
			BinaryPath: "sardis",
			Type:       jaspercli.RPCService,
			Port:       h.Port,
		}

		return jaspercli.NewSSHClient(remoteOpts, clientOpts, true)
	case "jasper":
		addrStr := fmt.Sprintf("%s:%d", h.Hostname, h.Port)

		serviceAddr, err := net.ResolveTCPAddr("tcp", addrStr)
		if err != nil {
			return nil, errors.Wrapf(err, "could not resolve Jasper service address at '%s'", addrStr)
		}

		dialCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		return rpc.NewClient(dialCtx, serviceAddr, nil)
	default:
		return nil, errors.New("unsupported jasper protocol")
	}
}
