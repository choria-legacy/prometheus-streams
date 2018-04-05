# choria/prometheus_streams

## Overview

The Choria Prometheus Streams Federation is a tool that polls Prometheus metrics and distribute them over NATS Streams.  On the Receiver they are published into Prometheus Push Gateway

## Module Description

The module installs, configures and manages the associated services.

In order to install the package you have to add the Choria YUM repository to your system, in future there will be a `choria` module to do this for you.

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

## Usage

```puppet
class{"prometheus_streams":
  poller => true,
  receiver => true,
  poller_stream => {
    cluster_id => "dc1_stream",
    urls => "nats://nats.dc1.example.net:4222",
    topic => "prometheus"
  },
  receiver_stream => {
    client_id => "prometheus_receiver",
    cluster_id => "global_stream",
    urls => "nats://nats.dc2.example.net:4222",
    topic => "prometheus"
  },
  push_gateway => {
    url => "http://prometheus.dc2.example.net:9091",
    publisher_label => true,
  },
  management => {
    identity => "prom.example.net",
    collective => "prometheus",
    brokers => ["choria.example.net:4222"]
  },
  jobs => {
    "choria" => {
      "targets" => [
        {
          "name" => "choria1",
          "url"=> "http://choria1.dc1.example.net:8222/choria/prometheus"
        }
      ]
    }
  }
}
```

Full reference about the available options for configuring topics can be found in the project documentation.