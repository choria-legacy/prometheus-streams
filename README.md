What?
=====

A federation system for Prometheus that sends metrics via NATS Streams rather than HTTP.

Why?
----

First let me say this is a bad idea, in general, just don't even think of using this.

Prometheus has it's own built in federation which, like Prometheus, is based on HTTP Polls.

If you want to have a central Prometheus it does HTTP polls to all remote Prometheus and present their data locally.  In general this is what you should do - yes, it's weird, but it's a good fit for the Prometheus model and it's your best bet.

However, in my site I am unable to open the 40+ intra DC HTTP ports this requires - I don't think it would even be allowed let alone the amount of paper work that could consume years.

I do already have a NATS Stream based middleware system that spans the globe, this system lets me piggy back on that globe spanning stream for sending my metrics to a central Prometheus.  Giving me a nice single instance for logging, metrics etc.

Status?
-------

This is a work in progress, once it's done I'll probably move it into the choria-io organization on GitHub.

Basic TODO:

  * Keep metrics
  * RPMs
  * Remove the Push Gateway dependency
  * Tests

Architecture?
-------------

The basic idea is that a poller/scraper will publish it's data into a NATS Stream and a Receiver will receiver the published data and push it into a [Push Gateway](https://github.com/prometheus/pushgateway).

```
 +----------+
 |Prometheus|
 +-----+----+
       ^
       |
       |
       |
       |
+------+-----+         +--------+       +-----------+
|Push Gateway| <-------+Receiver| <-----+NATS Stream|
+------------+         +--------+       +------+----+
                                               ^
                                               |
                                               |
                                               |
                                           +---+--+
                                           |Poller|
                                           +---+--+
                                               ^
                                               |
                                               |
                                               |
                                               |
                                      +--------+--------+
                                      |Monitored Systems|
                                      +-----------------+

```

At first this looks like an awful Rube Goldberg machine - and you're probably right, read the _Why?_ section - but this does avoid the problem with HTTP port from a central DC to all other DCs.  If you're in a big enterprise you'll appreciate this HTTP port is near impossible to provision.

In practice this works remarkably well, even if I use the [Choria Stream Replicator](https://github.com/choria-io/stream-replicator) to shift these metrics globally it all works quite well and gives me sub 1 minute monitoring resolution which is very very much better than the alternative enterprise monitoring systems I have at my disposal.

I deploy many Poller instances - 1 per data centre - and 1 receiver to put the data into the Push Gateway.  For my use case of ~10 monitored targets per DC this is more than sufficient.

Configuration?
--------------

A sample config can be seen here:

```yaml
---
verbose: true
debug: false
logfile: /var/log/prometheus-streams.log

# scrape every minute
scrape_interval: 60s

# the receiver will just delete scrapes thats older than 80 seconds
max_age: 80

# the poller will publish into this NATS Stream
poller_stream:
  cluster_id: dc1_stream
  urls: nats://nats.dc1.example.net:4222
  topic: prometheus

# the receiver will read here, the client_id has to be unique
receiver_stream:
  client_id: prometheus_receiver
  cluster_id: global_stream
  urls: nats://nats.dc2.example.net:4222
  topic: prometheus

# the receiver will push to this Push Gateway
push_gateway:
  url: http://prometheus.dc2.example.net:9091

# This is what gets polled
jobs:
  # a choria job, this maps into the push gateway URL
  choria:
      # with many instances, each instance gets a label matching it's name
      # else the name match the host:port of the url like prometheus would
      targets:
        - name: choria1
          url: http://choria1.dc1.example.net:8222/choria/prometheus
        - name: choria2
          url: http://choria2.dc1.example.net:8222/choria/prometheus
        - name: choria3
          url: http://choria3.dc1.example.net:8222/choria/prometheus
```

Contact?
--------

rip@devco.net / @ripienaar / https://www.devco.net/