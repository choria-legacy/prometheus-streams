package agent

import (
	"context"
	"fmt"

	"github.com/choria-io/prometheus-streams/build"
	"github.com/choria-io/prometheus-streams/management/facts"
	"github.com/choria-io/prometheus-streams/receiver"
	"github.com/choria-io/prometheus-streams/scrape"

	"github.com/choria-io/go-choria/choria"
	"github.com/choria-io/go-choria/mcorpc"
	"github.com/choria-io/go-choria/server"
	"github.com/choria-io/go-choria/server/agents"
	"github.com/sirupsen/logrus"
)

type infoReply struct {
	Mode   string `json:"mode"`
	Paused bool   `json:"paused"`
}

func Register(ctx context.Context, fw *choria.Framework, cserver *server.Instance) error {
	// mcollective like metadata
	m := &agents.Metadata{
		Name:        "prometheus_streams",
		Description: "Prometheus Streams",
		Author:      "R.I.Pienaar <rip@devco.net>",
		Version:     build.Version,
		License:     "Apache-2.0",
		Timeout:     1,
		URL:         "http://choria.io",
	}

	agent := mcorpc.New("prometheus_streams", m, fw, logrus.WithFields(logrus.Fields{"agent": "prometheus_streams"}))

	agent.MustRegisterAction("switch", switchAction)
	agent.MustRegisterAction("info", infoAction)

	// adds the agent to the running instance of the server
	// this has to happen after its initial connect
	return cserver.RegisterAgent(ctx, "prometheus_streams", agent)
}

func switchAction(ctx context.Context, req *mcorpc.Request, reply *mcorpc.Reply, agent *mcorpc.Agent, conn choria.ConnectorInfo) {
	receiver.FlipCircuitBreaker()
	scrape.FlipCircuitBreaker()

	infoAction(ctx, req, reply, agent, conn)

	err := facts.Write()
	if err != nil {
		reply.Statuscode = mcorpc.Aborted
		reply.Statusmsg = fmt.Sprintf("Could not update facts: %s", err)
	}
}

func infoAction(ctx context.Context, req *mcorpc.Request, reply *mcorpc.Reply, agent *mcorpc.Agent, conn choria.ConnectorInfo) {
	reply.Data = getInfo()
}

func getInfo() *infoReply {
	r := infoReply{
		Mode:   facts.Mode(),
		Paused: facts.Paused(),
	}

	return &r
}
