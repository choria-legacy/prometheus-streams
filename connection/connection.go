package connection

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/choria-io/prometheus-streams/backoff"
	"github.com/choria-io/prometheus-streams/config"
	uuid "github.com/gofrs/uuid"
	nats "github.com/nats-io/go-nats"
	stan "github.com/nats-io/go-nats-streaming"
	log "github.com/sirupsen/logrus"
)

type Connection struct {
	ctx  context.Context
	name string
	cid  string
	urls string
	Conn stan.Conn
	nc   *nats.Conn
	tlsc *tls.Config
}

func NewConnection(ctx context.Context, cfg *config.StreamConfig, cb func(c stan.Conn, reason error)) (*Connection, error) {
	if cfg.ClientID == "" {
		id, err := uuid.NewV4()
		if err != nil {
			return nil, err
		}

		cfg.ClientID = fmt.Sprintf("prometheus_streams_%s", id.String())
	}

	c := Connection{
		ctx:  ctx,
		name: cfg.ClientID,
		cid:  cfg.ClusterID,
		urls: cfg.URLs,
	}

	if cfg.TLS != nil {
		prov, err := cfg.TLS.SecurityProvider()
		if err != nil {
			return nil, fmt.Errorf("could not initiate security system: %s", err)
		}

		c.tlsc, err = prov.TLSConfig()
		if err != nil {
			return nil, fmt.Errorf("could not configure TLS settings: %s", err)
		}
	}

	c.connect(cb)

	return &c, nil
}

func (c *Connection) connect(cb func(c stan.Conn, reason error)) {
	c.Conn = c.connectSTAN(cb)
}

func (c *Connection) Close() {
	err := c.Conn.Close()
	if err != nil {
		log.Errorf("Could not close Stream connection, ignoring: %s", err)
	}

	c.nc.Close()
}

func (c *Connection) Publish(target string, body []byte) error {
	if c.Conn == nil {
		return fmt.Errorf("not connected")
	}

	return c.Conn.Publish(target, body)
}

// PublishRaw implements lifecycle.PublishConnector
func (c *Connection) PublishRaw(target string, body []byte) error {
	return c.Publish(target, body)
}

func (c *Connection) connectSTAN(cb func(stan.Conn, error)) stan.Conn {
	c.nc = c.connectNATS()
	if c.nc == nil {
		log.Errorf("%s NATS connection could not be established, cannot connect to the Stream", c.name)
		return nil
	}

	var err error
	var conn stan.Conn
	try := 0

	for {
		try++

		conn, err = stan.Connect(c.cid, c.name, stan.NatsConn(c.nc), stan.SetConnectionLostHandler(cb))
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

	if c.tlsc != nil {
		options = append(options, nats.Secure(c.tlsc))
	}

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
