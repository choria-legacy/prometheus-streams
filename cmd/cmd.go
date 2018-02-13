package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/choria-io/prometheus-streams/build"
	"github.com/choria-io/prometheus-streams/config"
	"github.com/choria-io/prometheus-streams/management"
	"github.com/choria-io/prometheus-streams/receiver"
	"github.com/choria-io/prometheus-streams/scrape"
	log "github.com/sirupsen/logrus"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	cfile   string
	pidfile string
	cfg     *config.Config
	err     error
	wg      *sync.WaitGroup
	ctx     context.Context
	cancel  func()
	debug   bool
)

// Run sets up the CLI and perform the users desired actions
func Run() {
	app := kingpin.New("prometheus-streams", "Prometheus NATS Streaming based poller and publisher")
	app.Version(build.Version)
	app.Author("R.I.Pienaar <rip@devco.net>")

	app.Flag("config", "Configuration file").Required().ExistingFileVar(&cfile)
	app.Flag("pid", "Write running PID to a file").StringVar(&pidfile)
	app.Flag("debug", "Force debug logging").BoolVar(&debug)

	p := app.Command("poller", "Polls for and published Prometheus metrics")
	r := app.Command("receiver", "Received and pushes Prometheus metrics published by the poller")

	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	cfg, err = parseCfg()
	if err != nil {
		kingpin.Fatalf("Failed to parse config: %s", err)
	}

	if debug {
		cfg.Debug = true
	}

	writePID(pidfile)
	configureLogging()

	wg = &sync.WaitGroup{}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	go interrupWatcher(cancel)

	switch cmd {
	case p.FullCommand():
		poll()
	case r.FullCommand():
		receive()
	}

	err = configureManagement()
	if err != nil {
		kingpin.Fatalf("Could not configure management: %s", err)
	}

	wg.Wait()
}

func parseCfg() (*config.Config, error) {
	return config.NewConfig(cfile)
}

func poll() {
	wg.Add(1)
	go scrape.Run(ctx, wg, cfg)
}

func receive() {
	wg.Add(1)
	go receiver.Run(ctx, wg, cfg)
}

func interrupWatcher(cancel func()) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigs:
		log.Infof("Shutting down on %s", sig)
		cancel()
	}
}

func configureLogging() {
	if cfg.LogFile != "" {
		log.SetFormatter(&log.JSONFormatter{})

		file, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatalf("Cannot open log file %s: %s", cfg.LogFile, err)
			os.Exit(1)
		}

		log.SetOutput(file)
	}

	log.SetLevel(log.InfoLevel)

	if cfg.Verbose {
		log.SetLevel(log.InfoLevel)
	}

	if cfg.Debug {
		log.SetLevel(log.DebugLevel)
	}
}

func configureManagement() error {
	if cfg.Management == nil {
		return nil
	}

	err := management.Configure(cfg)
	if err != nil {
		return err
	}

	wg.Add(1)
	go management.Run(ctx, wg)

	return nil
}

func writePID(pidfile string) {
	if pidfile == "" {
		return
	}

	err := ioutil.WriteFile(pidfile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	if err != nil {
		kingpin.Fatalf("Could not write PID: %s", err)
		os.Exit(1)
	}
}
