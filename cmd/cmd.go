package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/choria-io/go-backplane/backplane"
	"github.com/choria-io/go-security/puppetsec"
	"github.com/choria-io/prometheus-streams/build"
	"github.com/choria-io/prometheus-streams/config"
	"github.com/choria-io/prometheus-streams/receiver"
	"github.com/choria-io/prometheus-streams/scrape"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	enrollIdentity string
	enrollCA       string
	enrollDir      string
)

// Run sets up the CLI and perform the users desired actions
func Run() {
	app := kingpin.New("prometheus-streams", "Prometheus NATS Streaming based poller and publisher")
	app.Version(build.Version)
	app.Author("R.I.Pienaar <rip@devco.net>")

	app.Flag("config", "Configuration file").ExistingFileVar(&cfile)
	app.Flag("pid", "Write running PID to a file").StringVar(&pidfile)
	app.Flag("debug", "Force debug logging").BoolVar(&debug)

	p := app.Command("poller", "Polls for and published Prometheus metrics")
	r := app.Command("receiver", "Received and pushes Prometheus metrics published by the poller")
	e := app.Command("enroll", "Enrolls with a Puppet CA")

	e.Arg("identity", "Certificate Name to use when enrolling").StringVar(&enrollIdentity)
	e.Flag("ca", "Host and port for the Puppet CA in host:port format").Default("puppet:8140").StringVar(&enrollCA)
	e.Flag("dir", "Directory to write SSL configuration to").Required().StringVar(&enrollDir)

	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	wg = &sync.WaitGroup{}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	if debug {
		log.SetLevel(log.DebugLevel)
	}

	if cmd == e.FullCommand() {
		enroll()
		return
	}

	cfg, err = parseCfg()
	if err != nil {
		kingpin.Fatalf("Failed to parse config: %s", err)
	}

	if debug {
		cfg.Debug = true
	}

	writePID(pidfile)
	configureLogging()

	go interrupWatcher(cancel)

	if cfg.MonitorPort > 0 {
		go setupPrometheus(cfg.MonitorPort)
	}

	switch cmd {
	case p.FullCommand():
		poll()
	case r.FullCommand():
		receive()
	}

	err = configureManagement()
	if err != nil {
		log.Errorf("Management did not start: %s", err)
	}

	wg.Wait()
}

func configureManagement() error {
	if cfg.Management == nil {
		return nil
	}

	opts := []backplane.Option{backplane.ManageInfoSource(cfg)}
	if receiver.Pausable != nil {
		opts = append(opts, backplane.ManagePausable(receiver.Pausable))
	} else if scrape.Pausable != nil {
		opts = append(opts, backplane.ManagePausable(scrape.Pausable))
	} else {
		return fmt.Errorf("neither scrap nor receiver are running, cannot start backplane")
	}

	_, err := backplane.Run(ctx, wg, cfg.Management, opts...)
	return err
}

func parseCfg() (*config.Config, error) {
	if cfile == "" {
		return nil, errors.New("no configuration file supplied using --config")
	}

	return config.NewConfig(cfile)
}

func enroll() {
	cfg := puppetsec.Config{
		Identity:   enrollIdentity,
		SSLDir:     enrollDir,
		DisableSRV: true,
	}

	re := regexp.MustCompile("^((([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\\-]*[a-zA-Z0-9])\\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\\-]*[A-Za-z0-9]))\\:(\\d+)$")

	if re.MatchString(enrollCA) {
		parts := strings.Split(enrollCA, ":")
		cfg.PuppetCAHost = parts[0]

		p, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Fatalf("Could not enroll with the Puppet CA: %s", err)
		}

		cfg.PuppetCAPort = p
	}

	prov, err := puppetsec.New(puppetsec.WithConfig(&cfg), puppetsec.WithLog(log.WithField("provider", "puppet")))
	if err != nil {
		log.Fatalf("Could not enroll with the Puppet CA: %s", err)
	}

	wait, _ := time.ParseDuration("30m")

	err = prov.Enroll(ctx, wait, func(try int) { fmt.Printf("Attempting to download certificate for %s, try %d.\n", enrollIdentity, try) })
	if err != nil {
		log.Fatalf("Could not enroll with the Puppet CA: %s", err)
	}
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

func setupPrometheus(port int64) {
	log.Infof("Listening for /metrics on %d", port)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
