type Prometheus_streams::Management = Struct[{
    brokers => Array[String],
    identity => Optional[String],
    collective => Optional[String],
    tls => Optional[Variant[Prometheus_streams::FileSSL, Prometheus_streams::PuppetSSL]]
}]
