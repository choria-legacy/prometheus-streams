# Choria NATS Streaming based federation for Prometheus
#
# Install and configure a tool to federate Prometheus metrics via NATS Streams
#
# This module does not install the Choria YUM repository, it's configured as:
#
# ```
# [choria_release]
# name=choria_release
# baseurl=https://packagecloud.io/choria/release/el/$releasever/$basearch
# repo_gpgcheck=1
# gpgcheck=0
# enabled=1
# gpgkey=https://packagecloud.io/choria/release/gpgkey
# sslverify=1
# sslcacert=/etc/pki/tls/certs/ca-bundle.crt
# metadata_expire=300
# ```
#
# @param poller Enables the poller service
# @param receiver Enables the receiver service
# @param config_file Path to the configuration file
# @param log_file Path to the log file
# @param debug Enables debug logging
# @param verbose Enables verbose logging
# @param scrape_interval Interval to scrape targets
# @param max_age Maximum age of metrics the receiver will accept
# @param monitor_port When set enables prometheus metrics on this port
# @param poller_stream Stream to publish metrics to
# @param receiver_stream Stream to read metrics from
# @param push_gateway Push Gateway the Receiver will publish metrics to
# @param management Configuration for the embedded Choria based management interface
# @param jobs Targets to poll
# @param poller_service_name The name of the Poller service
# @param receiver_service_name The name of the Receiver service
# @param package_name The name of the package to install
# @param ensure To install or uninstall the software
# @param version Which version to install
class prometheus_streams (
    Boolean $poller = false,
    Boolean $receiver = false,
    Stdlib::Absolutepath $config_file = "/etc/prometheus-streams/prometheus-streams.yaml",
    Stdlib::Absolutepath $log_file = "/var/log/prometheus-streams.log",
    Boolean $debug = false,
    Boolean $verbose = false,
    Pattern[/^\d+(m|h|s)$/] $scrape_interval = "30s",
    Integer $max_age = 65,
    Integer $monitor_port = 0,
    Variant[Prometheus_streams::Stream, Hash[0, 0]] $poller_stream = {},
    Variant[Prometheus_streams::Stream, Hash[0, 0]] $receiver_stream = {},
    Variant[Prometheus_streams::Push_gateway, Hash[0, 0]] $push_gateway = {},
    Variant[Prometheus_streams::Management, Hash[0, 0]] $management = {},
    Hash[String, Prometheus_streams::Job] $jobs = {},
    String $poller_service_name = "prometheus-streams-poller",
    String $receiver_service_name = "prometheus-streams-receiver",
    String $package_name = "prometheus-streams",
    Enum[present, absent] $ensure = "present",
    String $version = "present"
) {
    if $ensure == "present" {
        class{"prometheus_streams::install": } ->
        class{"prometheus_streams::config": } ~>
        class{"prometheus_streams::service": }
    } else {
        class{"prometheus_streams::service": } ->
        class{"prometheus_streams::install": }
    }
}