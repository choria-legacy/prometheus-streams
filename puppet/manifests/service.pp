class prometheus_streams::service {
   if $prometheus_streams::ensure == "present" {
        $_sensure = "running"
        $_senable = true
    } else {
        $_sensure = "stopped"
        $_senable = false
    }

    if $prometheus_streams::poller {
        service{$prometheus_streams::poller_service_name:
            ensure => $_sensure,
            enable => $_senable
        }
    }

    if $prometheus_streams::receiver {
        service{$prometheus_streams::receiver_service_name:
            ensure => $_sensure,
            enable => $_senable
        }
    }
}