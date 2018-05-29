type Prometheus_streams::PuppetSSL = Struct[{
  identity => String,
  scheme => Enum["puppet"],
  ssl_dir => Stdlib::Unixpath
}]
