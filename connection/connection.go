package connection

import (
	"context"
	"fmt"
	"time"

	"github.com/apex/log"
	"github.com/choria-io/stream-replicator/backoff"
	nats "github.com/nats-io/go-nats"
	"github.com/nats-io/go-nats-streaming"
	"github.com/ripienaar/prometheus-streams/config"
	uuid "github.com/satori/go.uuid"
)

type Connection struct {
	ctx  context.Context
	name string
	cid  string
	urls string
	Conn stan.Conn
}

func NewConnection(ctx context.Context, cfg *config.StreamConfig) *Connection {
	if cfg.ClientID == "" {
		cfg.ClientID = fmt.Sprintf("prometheus_streams_%s", uuid.NewV1().String())
	}

	c := Connection{
		ctx:  ctx,
		name: cfg.ClientID,
		cid:  cfg.ClusterID,
		urls: cfg.URLs,
	}

	c.Connect()

	return &c
}

func (c *Connection) Connect() {
	c.Conn = c.connectSTAN()
}

func (c *Connection) Close() {
	c.Conn.Close()
}

func (c *Connection) Publish(target string, body []byte) error {
	return c.Conn.Publish(target, body)
}

func (c *Connection) connectSTAN() stan.Conn {
	n := c.connectNATS()
	if n == nil {
		log.Errorf("%s NATS connection could not be established, cannot connect to the Stream", c.name)
		return nil
	}

	var err error
	var conn stan.Conn
	try := 0

	for {
		try++

		conn, err = stan.Connect(c.cid, c.name, stan.NatsConn(n))
		if err != nil {
			log.Warnf("%s initial connection to the NATS Streaming broker cluster failed: %s", c.name, err)

			if c.ctx.Err() != nil {
				log.Errorf("%s initial connection cancelled due to shut down", c.name)
				return nil
			}

			log.Infof("%s NATS Stream client failed connection attempt %d", c.name, try)

			if backoff.FiveSec.InterruptableSleep(c.ctx, try) != nil {
				return nil
			}

			continue
		}

		break
	}

	return conn
}

func (c *Connection) connectNATS() (natsc *nats.Conn) {
	options := []nats.Option{
		nats.MaxReconnects(-1),
		nats.Name(c.name),
		nats.DisconnectHandler(c.disconCb),
		nats.ReconnectHandler(c.reconCb),
		nats.ClosedHandler(c.closedCb),
		nats.ErrorHandler(c.errorCb),
	}

	var err error
	try := 0

	for {
		try++

		natsc, err = nats.Connect(c.urls, options...)
		if err != nil {
			log.Warnf("%s initial connection to the NATS broker cluster failed: %s", c.name, err)

			if c.ctx.Err() != nil {
				log.Errorf("%s initial connection cancelled due to shut down", c.name)
				return nil
			}

			s := backoff.FiveSec.Duration(try)
			log.Infof("%s NATS client sleeping %s after failed connection attempt %d", c.name, s, try)

			timer := time.NewTimer(s)

			select {
			case <-timer.C:
				continue
			case <-c.ctx.Done():
				log.Errorf("%s initial connection cancelled due to shut down", c.name)
				return nil
			}
		}

		log.Infof("%s NATS client connected to %s", c.name, natsc.ConnectedUrl())

		break
	}

	return
}

func (c *Connection) disconCb(nc *nats.Conn) {
	err := nc.LastError()

	if err != nil {
		log.Warnf("%s NATS client connection got disconnected: %s", nc.Opts.Name, err)
	} else {
		log.Warnf("%s NATS client connection got disconnected", nc.Opts.Name)
	}
}

func (c *Connection) reconCb(nc *nats.Conn) {
	log.Warnf("%s NATS client reconnected after a previous disconnection, connected to %s", nc.Opts.Name, nc.ConnectedUrl())
}

func (c *Connection) closedCb(nc *nats.Conn) {
	err := nc.LastError()

	if err != nil {
		log.Warnf("%s NATS client connection closed: %s", nc.Opts.Name, err)
	} else {
		log.Warnf("%s NATS client connection closed", nc.Opts.Name)
	}

}

func (c *Connection) errorCb(nc *nats.Conn, sub *nats.Subscription, err error) {
	log.Errorf("%s NATS client on %s encountered an error: %s", nc.Opts.Name, nc.ConnectedUrl(), err)
}
