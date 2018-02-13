class prometheus_streams::install {
    if $prometheus_streams::ensure == "present" {
        $ensure = $prometheus_streams::version
    } else {
        $ensure = "absent"
    }

    package{$prometheus_streams::package_name:
        ensure => $ensure
    }
}