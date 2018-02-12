package facts

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/choria-io/go-choria/choria"
	"github.com/choria-io/prometheus-streams/config"
	"github.com/choria-io/prometheus-streams/receiver"
	"github.com/choria-io/prometheus-streams/scrape"
	"github.com/sirupsen/logrus"
)

var f *os.File
var cconf *choria.Config
var sconf *config.Config
var err error

type FactData struct {
	Identity       string          `json:"identity"`
	Time           int64           `json:"time"`
	Config         json.RawMessage `json:"config"`
	PollerPaused   bool            `json:"poller_paused"`
	ReceiverPaused bool            `json:"receiver_paused"`
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
		err := write()
		if err != nil {
			logrus.Warnf("Could not write fact data: %s", err)
		}
	}

	logrus.Infof("Writing fact data to %s", f.Name())

	writer()

	for {
		select {
		case <-time.Tick(time.Duration(60) * time.Second):
			writer()
		case <-ctx.Done():
			return
		}
	}
}

// Data returns current set of Factdata
func Data() FactData {
	j, _ := json.Marshal(sconf)

	return FactData{
		Identity:       sconf.Management.Identity,
		Time:           time.Now().UTC().Unix(),
		Config:         json.RawMessage(j),
		PollerPaused:   scrape.Paused(),
		ReceiverPaused: receiver.Paused(),
	}
}

func write() error {
	tf, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	defer os.Remove(tf.Name())

	tf.Close()

	j, err := json.Marshal(Data())
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(tf.Name(), j, 0644)
	if err != nil {
		return err
	}

	err = os.Rename(tf.Name(), f.Name())
	if err != nil {
		return err
	}

	return nil
}
