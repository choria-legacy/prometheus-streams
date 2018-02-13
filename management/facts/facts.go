package facts

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/choria-io/go-choria/choria"
	"github.com/choria-io/prometheus-streams/config"
	"github.com/choria-io/prometheus-streams/receiver"
	"github.com/choria-io/prometheus-streams/scrape"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/sjson"
)

var f *os.File
var cconf *choria.Config
var sconf *config.Config
var err error
var mu = &sync.Mutex{}

type FactData struct {
	Identity string          `json:"identity"`
	Time     int64           `json:"time"`
	Config   json.RawMessage `json:"config"`
	Paused   bool            `json:"paused"`
	Mode     string          `json:"mode"`
	Jobs     string          `json:"jobs"`
}

func Configure(c *config.Config) {
	sconf = c
}

func Expose(ctx context.Context, wg *sync.WaitGroup, cfg *choria.Config) {
	defer wg.Done()

	cconf = cfg

	f, err = ioutil.TempFile("", "prometheus-stream-facts")
	if err != nil {
		logrus.Fatalf("Could not create fact temp file: %s", err)
	}
	defer os.Remove(f.Name())

	// we will do atomic write to this file via renames, so not directly to it
	f.Close()

	// discovery will find this file now and report on its contents as json format
	cfg.FactSourceFile = f.Name()

	writer := func() {
		err := Write()
		if err != nil {
			logrus.Warnf("Could not write fact data: %s", err)
		}
	}

	logrus.Infof("Writing fact data to %s", f.Name())

	writer()

	for {
		select {
		case <-time.Tick(time.Duration(600) * time.Second):
			writer()
		case <-ctx.Done():
			return
		}
	}
}

// Data returns current set of Factdata
func data() json.RawMessage {
	j, _ := json.Marshal(sconf)

	f := FactData{
		Identity: sconf.Management.Identity,
		Time:     time.Now().UTC().Unix(),
		Config:   json.RawMessage(j),
		Mode:     Mode(),
		Paused:   Paused(),
	}

	j, _ = json.Marshal(f)
	j, _ = sjson.SetBytes(j, fmt.Sprintf("%s_mode", Mode()), true)

	if scrape.Running() {
		jobs := []string{}
		for job := range sconf.Jobs {
			jobs = append(jobs, job)
		}

		j, _ = sjson.SetBytes(j, "jobs", strings.Join(jobs, ","))

		for _, job := range jobs {
			j, _ = sjson.SetBytes(j, fmt.Sprintf("%s_job", job), true)
		}
	}

	return json.RawMessage(j)
}

func Write() error {
	mu.Lock()
	defer mu.Unlock()

	tf, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	defer os.Remove(tf.Name())

	tf.Close()

	err = ioutil.WriteFile(tf.Name(), data(), 0644)
	if err != nil {
		return err
	}

	err = os.Rename(tf.Name(), f.Name())
	if err != nil {
		return err
	}

	return nil
}

func Mode() string {
	if scrape.Running() {
		return "poller"
	}

	if receiver.Running() {
		return "receiver"
	}

	return "unknown"
}

func Paused() bool {
	if scrape.Running() {
		return scrape.Paused()
	}

	if receiver.Running() {
		return receiver.Paused()
	}

	return false
}
