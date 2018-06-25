NATS Streaming based federation for Prometheus
==============================================

A federation system for Prometheus that sends metrics via NATS Streams and into the Push Gateway.

Overview
--------

First let me say this is a bad idea, in general, just don't even think of using this. You have to have the exact same problem this solves else you will be better off just using Prometheus as they recommend.

Prometheus has it's own built in federation which, like Prometheus, is based on HTTP Polls. A central Prometheus polls the data from each federated Prometheus - and each of those in turn does their own polling. In general this is what you should do - yes, it's weird, but it's a good fit for the Prometheus model and it's your best bet.

In many sites though users are unable to open the intra DC HTTP ports this requires - in my case I don't think it would even be allowed let alone the amount of paper work that could consume years.

I do already have a NATS Stream based middleware system that spans the globe, this system lets me piggy back on that globe spanning stream for sending my metrics to a central Prometheus.  Giving me a nice single instance for logging, metrics etc.

When To Use
-----------

You should have all these criteria:

 - [ ] You cannot use the standard Prometheus Federation
 - [ ] You do not have 100s of devices in every DC to monitor, 10s is what this is good at
 - [ ] You understand the issues related to the [Push Gateway](https://github.com/prometheus/pushgateway) and you are happy to use it to expose the metrics to your central Prometheus
 - [ ] You looked at [PushProx](https://github.com/RobustPerception/PushProx) and decided not to use it - perhaps you also can't make extra ports to your central DC
 - [ ] You already have or do not have a problem running [NATS Streaming Server](https://github.com/nats-io/nats-streaming-server).  You can have one central Stream or many stitched together with the [Stream Replicator](https://github.com/choria-io/stream-replicator)

If all this is true, go ahead and look to this tool for a possible solution to your problem.

Architecture
------------

The basic idea is that a poller/scraper will publish it's data into a NATS Stream and a Receiver will receiver the published data and push it into a [Push Gateway](https://github.com/prometheus/pushgateway).

```
 +----------+
 |Prometheus|
 +-----+----+
       ^
       |
+------+-----+         +--------+       +-----------+       +---+--+
|Push Gateway| <-------+Receiver| <-----+NATS Stream| <-----+Poller|
+------------+         +--------+       +------+----+       +---+--+
                                                                ^
                                                                |
                                                       +--------+--------+
                                                       |Monitored Systems|
                                                       +-----------------+

```

At first this looks like an awful Rube Goldberg machine - and you're probably right, read the _Overview_ section - but this does avoid the problem with HTTP port from a central DC to all other DCs.  If you're in a big enterprise you'll appreciate this HTTP port is near impossible to provision.

In practice this works remarkably well, even if I use the [Choria Stream Replicator](https://github.com/choria-io/stream-replicator) to shift these metrics globally it all works quite well and gives me sub 1 minute monitoring resolution which is very very much better than the alternative enterprise monitoring systems I have at my disposal.

I deploy many Poller instances - 1 per data centre - and 1 receiver to put the data into the Push Gateway.  For my use case of ~10 monitored targets per DC this is more than sufficient.

Requirements
------------

To get the most stable long running connection to the NATS Streaming Server you should use at least version 0.10.0 of the Streaming Server.

Configuration
-------------

A sample config can be seen here:

```yaml
---
verbose: true
debug: false
logfile: /var/log/prometheus-streams.log

# when publishing stats this will be reported as the publisher
# defaults to the hostname reported by the operating system
identity: USDC1

# scrape every minute
scrape_interval: 60s

# the receiver will just delete scrapes thats older than 80 seconds
max_age: 80

# sets up a prometheus stats listener for /metrics on port 1000
monitor_port: 10000

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

  # when true labels will be added with the hostname of the publisher
  publisher_label: true

# enable a choria based management interface for circuit breaking
management:
  identity: prom.example.net # defaults to hostname
  collective: prometheus # the default
  brokers:
    - choria.example.net:4222

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

I set my Stream to keep 10 minutes of data only for this data everywhere.

Configure Prometheus to consume data from the Push Gateway - here http://prometheus.dc2.example.net:9091/metrics.

TLS
---

TLS is supported on the NATS and management connections, you can configure a single TLS setup for all connections or per connection. For all connections add this to the top of the config, else you can add the exact configuration in the `management`, `receiver_stream` and `poller_stream` sections, this is handy if they have different CAs.

2 modes are supported, Puppet CA compatible or full manual configuration:

### Puppet

In this mode a Puppet compatible SSL setup is required in a specific directory:

```yaml
tls:
  identity: prom.example.net
  ssl_dir: /etc/prometheus-streams/ssl
  scheme: puppet
```

If you use this mode you can use the `prometheus-stream enroll prom.example.net --dir /etc/prometheus-streams/ssl prom.example.net` command to go through the process of obtaining a certificate for this instance from the PuppetCA, the process is identical to the `--waitforcert` mode in `puppet agent` and requires you to have a unique certificate identity.

### Manual

This mode is completely configurable:

```yaml
tls:
  identity: prom.example.net
  scheme: manual
  ca: /path/to/ca.pem
  cert: /path/to/cert.pem
  key: /path/to/key.pem
  cache: /path/to/cert_cache # required if you use the management backend
```

These `tls` stanzas can be set either at the top level as here - where it will apply to all NATS connections - or on the individual `management`, `receiver_stream` and `poller_stream` level in the event that you need different set ups for these.

Own Metrics
-----------

When `monitor_port` is set to a value greater than 0 this process will listen for Prometheus requests on `/metrics` on that port.

It will also add itself as a scrape job called `prometheus_streams` and set up a matching target so you do not need to set up a scrape for itself when metrics are enabled.

Management
----------

A Choria server is embedded that can be used for management capabilities, this include extracting current information about the process and a circuit breaker to stop all processing.

To enable it provide a *management* block in the configuration else it will be ignored.

```
$ mco rpc prometheus_streams switch -T prometheus
Discovering hosts using the mc method for 2 second(s) .... 1

 * [ ============================================================> ] 1 / 1


prom.example.net
     Mode: poller
   Paused: true


Summary of Mode:

   poller = 1

Summary of Paused:

   false = 1

Finished processing 1 / 1 hosts in 399.81 ms
```

At this point all processing will stop, no polling will be done and received messages will be discarded.  Switch it again to enable.

The `info` action returns the same data.

You can extract the running configuration using something like `mco rpc rpcutil get_fact fact=config -T prometheus`

**NOTE:** TLS is not currently supported, this feature is basically a bit of a work in progress / experiment.

Packages
--------

RPMs for EL6 and 7 are published in the choria YUM repository, packages are called `prometheus-streams`.

```ini
[choria_release]
name=choria_release
baseurl=https://packagecloud.io/choria/release/el/$releasever/$basearch
repo_gpgcheck=1
gpgcheck=0
enabled=1
gpgkey=https://packagecloud.io/choria/release/gpgkey
sslverify=1
sslcacert=/etc/pki/tls/certs/ca-bundle.crt
metadata_expire=300
```

## Puppet Module

A Puppet module to install and manage the Stream Replicator can be found on the Puppet Forge as [choria/prometheus_streams](https://forge.puppet.com/choria/prometheus_streams)

## Thanks

<img src="https://packagecloud.io/images/packagecloud-badge.png" width="158">