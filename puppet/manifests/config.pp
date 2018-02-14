class prometheus_streams::config {
    $base_config = {
        "verbose" => $prometheus_streams::verbose,
        "debug" => $prometheus_streams::debug,
        "logfile" => $prometheus_streams::log_file,
        "monitor_port" => $prometheus_streams::monitor_port,
        "scrape_interval" => $prometheus_streams::scrape_interval,
        "max_age" => $prometheus_streams::max_age,
        "poller_stream" => $prometheus_streams::poller_stream,
        "receiver_stream" => $prometheus_streams::receiver_stream,
        "push_gateway" => $prometheus_streams::push_gateway,
        "jobs" => $prometheus_streams::jobs,
    }

    if !$prometheus_streams::management.empty {
        $config = $base_config + {"management" => $prometheus_streams::management}
    } else {
        $config = $base_config
    }

    file{$prometheus_streams::config_file:
        ensure => $prometheus_streams::ensure,
        content => to_yaml($config)
    }
}