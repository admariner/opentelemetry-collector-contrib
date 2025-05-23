# Failover Connector

<!-- status autogenerated section -->
| Status        |           |
| ------------- |-----------|
| Distributions | [contrib], [k8s] |
| Issues        | [![Open issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aopen%20label%3Aconnector%2Ffailover%20&label=open&color=orange&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aopen+is%3Aissue+label%3Aconnector%2Ffailover) [![Closed issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aclosed%20label%3Aconnector%2Ffailover%20&label=closed&color=blue&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aclosed+is%3Aissue+label%3Aconnector%2Ffailover) |
| Code coverage | [![codecov](https://codecov.io/github/open-telemetry/opentelemetry-collector-contrib/graph/main/badge.svg?component=connector_failover)](https://app.codecov.io/gh/open-telemetry/opentelemetry-collector-contrib/tree/main/?components%5B0%5D=connector_failover&displayType=list) |
| [Code Owners](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md#becoming-a-code-owner)    | [@akats7](https://www.github.com/akats7), [@fatsheep9146](https://www.github.com/fatsheep9146) |

[alpha]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[k8s]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-k8s

## Supported Pipeline Types

| [Exporter Pipeline Type] | [Receiver Pipeline Type] | [Stability Level] |
| ------------------------ | ------------------------ | ----------------- |
| traces | traces | [alpha] |
| metrics | metrics | [alpha] |
| logs | logs | [alpha] |

[Exporter Pipeline Type]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/connector/README.md#exporter-pipeline-type
[Receiver Pipeline Type]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/connector/README.md#receiver-pipeline-type
[Stability Level]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#stability-levels
<!-- end autogenerated section -->

Allows for health based routing between trace, metric, and log pipelines depending on the health of target downstream exporters.

## Configuration

If you are not already familiar with connectors, you may find it helpful to first visit the [Connectors README].

The following settings are available:

- `priority_levels (required)`: list of pipeline level priorities in a 1 - n configuration, multiple pipelines can sit at a single priority level.
- `retry_interval (optional)`: the frequency at which the pipeline levels will attempt to reestablish connection with all higher priority levels. Default value is 10 minutes. (See Example below for further explanation)
- `retry_gap (optional)`: * **Deprecated** * the amount of time between trying two separate priority levels in a single retry_interval timeframe. Default value is 30 seconds. (See Example below for further explanation)
- `max_retries (optional)`: **Deprecated** * the maximum retries per level. Default value is 10. Set to 0 to allow unlimited retries.

The connector intakes a list of `priority_levels` each of which can contain multiple pipelines.
If any pipeline at a stable level fails, the level is considered unhealthy and the connector will move down one priority level and route all data to the new level (assuming it is stable).

The connector will periodically try to reestablish a stable connection with the higher priority levels. `retry_interval` will be the frequency at which the connector will try to iterate through all unhealthy higher priority levels.

#### Configuration Example:

```yaml
connectors:
  failover:
    priority_levels:
      - [traces/first, traces/also_first]
      - [traces/second]
      - [traces/third]
    retry_interval: 10s

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [failover]
    traces/first:
      receivers: [failover]
      exporters: [otlp/first]
    traces/second:
      receivers: [failover]
      exporters: [otlp/second]
    traces/third:
      receivers: [failover]
      exporters: [otlp/third]
    traces/also_first:
      receivers: [failover]
      exporters: [otlp/fourth]
```

[Connectors README]:https://github.com/open-telemetry/opentelemetry-collector/blob/main/connector/README.md
[Exporter Pipeline Type]:https://github.com/open-telemetry/opentelemetry-collector/blob/main/connector/README.md#exporter-pipeline-type
[Receiver Pipeline Type]:https://github.com/open-telemetry/opentelemetry-collector/blob/main/connector/README.md#receiver-pipeline-type
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
