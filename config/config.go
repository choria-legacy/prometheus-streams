package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"time"

	"github.com/choria-io/go-backplane/backplane"
	"github.com/ghodss/yaml"
)

// Config configures the targets to scrape
type Config struct {
	Hostname string `json:"identity"`

	Verbose bool   `json:"verbose"`
	Debug   bool   `json:"debug"`
	LogFile string `json:"logfile"`

	Interval    string `json:"scrape_interval"`
	MaxAge      int64  `json:"max_age"`
	MonitorPort int64  `json:"monitor_port"`

	Jobs           map[string]*Job
	PollerStream   *StreamConfig                    `json:"poller_stream"`
	ReceiverStream *StreamConfig                    `json:"receiver_stream"`
	PushGateway    *PushGatewayConfig               `json:"push_gateway"`
	Management     *backplane.StandardConfiguration `json:"management"`

	TLS *TLSConf `json:"tls"`

	ConfigFile string `json:"-"`
}

// Job holds a specific job with many targets
type Job struct {
	Targets []*Target `json:"targets"`
}

// Target holds a specific target
type Target struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// StreamConfig is the target to publish data to
type StreamConfig struct {
	ClientID  string   `json:"client_id"`
	ClusterID string   `json:"cluster_id"`
	URLs      string   `json:"urls"`
	Topic     string   `json:"topic"`
	TLS       *TLSConf `json:"tls"`
}

// PushGatewayConfig where the receiver will publish metrics to
type PushGatewayConfig struct {
	URL            string `json:"url"`
	PublisherLabel bool   `json:"publisher_label"`
}

// NewConfig parses a config file into a Config
func NewConfig(file string) (*Config, error) {
	j, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	j, err = yaml.YAMLToJSON(j)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	err = json.Unmarshal(j, cfg)
	if err != nil {
		return nil, err
	}

	if cfg.Hostname == "" {
		cfg.Hostname, err = os.Hostname()
		if err != nil {
			return nil, err
		}
	}

	cfg.ConfigFile = file

	err = cfg.prepare()

	return cfg, err
}

// checks all urls are valid and set empty names to the host:port of the url
// when not specifically set
func (cfg *Config) prepare() error {
	_, err := time.ParseDuration(cfg.Interval)
	if err != nil {
		return err
	}

	if cfg.MonitorPort > 0 {
		t := []*Target{}
		t = append(t, &Target{
			Name: cfg.Hostname,
			URL:  fmt.Sprintf("http://localhost:%d/metrics", cfg.MonitorPort),
		})

		cfg.Jobs["prometheus_streams"] = &Job{
			Targets: t,
		}
	}

	if cfg.TLS != nil {
		if cfg.PollerStream.TLS == nil {
			cfg.PollerStream.TLS = cfg.TLS
		}

		if cfg.ReceiverStream.TLS == nil {
			cfg.ReceiverStream.TLS = cfg.TLS
		}

		if cfg.Management != nil && cfg.Management.TLSConf == nil {
			cfg.Management.TLSConf = &backplane.TLSConf{
				CA:       cfg.TLS.CA,
				Cache:    cfg.TLS.Cache,
				Cert:     cfg.TLS.Cert,
				Identity: cfg.TLS.Identity,
				Key:      cfg.TLS.Key,
				SSLDir:   cfg.TLS.SSLDir,
				Scheme:   cfg.TLS.Scheme,
			}
		}
	}

	for _, job := range cfg.Jobs {
		for _, target := range job.Targets {
			if target.Name == "" {
				u, err := url.Parse(target.URL)
				if err != nil {
					return err
				}

				target.Name = fmt.Sprintf("%s:%s", u.Hostname(), u.Port())
			}
		}
	}

	return nil
}
