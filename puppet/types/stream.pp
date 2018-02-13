type Prometheus_streams::Stream = Struct[{
    client_id => Optional[String],
    cluster_id => String,
    urls => String,
    topic => String
}]