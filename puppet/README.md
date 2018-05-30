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

### TLS

As of version 0.2.0 of the `prometheus-streams` package TLS is supported for all NATS connections and can be configured using this module.

While this module support configuring TLS properties such as paths to certificates it cannot create the certificate for you, you have to arrange another means of delivering the SSL keys, certificates etc to the host - perhaps in your profile class.

If you use the Puppet scheme you can configure it as below and use the `prometheus-streams enroll` command to create the SSL files:

```puppet
class{"prometheus_streams":
  tls                     => {
    "identity"            => $facts["fqdn"],
    "scheme"              => "puppet",
    "ssl_dir"             => "/etc/stream-replicator/ssl"
  },
  jobs                    => {
    # as above
  }
}
```

If you have another CA you can configure it manually, the `cache` is used by the management interface only:

```puppet
class{"prometheus_streams":
  tls                     => {
    "identity"            => $facts["fqdn"],
    "scheme"              => "manual",
    "ca"                  => "/path/to/ca.pem",
    "cert"                => "/path/to/cert.pem",
    "key"                 => "/path/to/key.pem",
    "cache"               => "/path/to/cache"
  },
  jobs                    => {
    # as above
  }
}
```

These `tls` stanzas can be set either at the top level as here - where it will apply to all NATS connections - or on the individual `management`, `receiver_stream` and `poller_stream` level in the event that you need different set ups for these.