type Prometheus_streams::Management = Struct[{
    brokers => Array[String],
    name => String,
    logfile => String,
    loglevel => Enum["debug", "info", "warn", "error", "fatal"],
    tls => Optional[Variant[Prometheus_streams::FileSSL, Prometheus_streams::PuppetSSL]],
    auth => Optional[Prometheus_streams::Authentication],
}]
