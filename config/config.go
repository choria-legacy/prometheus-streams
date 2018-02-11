package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"time"

	"github.com/ghodss/yaml"
)

// Config configures the targets to scrape
type Config struct {
	Verbose bool   `json:"verbose"`
	Debug   bool   `json:"debug"`
	LogFile string `json:"logfile"`

	Interval string `json:"scrape_interval"`
	MaxAge   int64  `json:"max_age"`

	Jobs           map[string]*Job
	PollerStream   *StreamConfig      `json:"poller_stream"`
	ReceiverStream *StreamConfig      `json:"receiver_stream"`
	PushGateway    *PushGatewayConfig `json:"push_gateway"`
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
	ClientID  string `json:"client_id"`
	ClusterID string `json:"cluster_id"`
	URLs      string `json:"urls"`
	Topic     string `json:"topic"`
}

type PushGatewayConfig struct {
	URL string `json:"url"`
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
