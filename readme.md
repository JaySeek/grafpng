
# Grafana reporter

A simple http service that generates *.png reports from [Grafana](http://grafana.org/) dashboards.

## Requirements
Build requirements:
* [golang](https://golang.org/)

## Getting started

### Generate a dashboard report

#### Endpoint

The reporter serves a png report on the specified port at:

    /api/v5/report/{dashboardUID}

where `{dashboardUID}` is the dashboard uid as used in the Grafana dashboard's URL.
E.g. `SoT6hL6zk` from `http://grafana-host:3000/d/SoT6hL6zk/descriptive-name`.
For more about this uid, see [the Grafana HTTP API](http://docs.grafana.org/http_api/dashboard/#identifier-id-vs-unique-identifier-uid).


#### Query parameters

The endpoint supports the following optional query parameters. These can be combined using standard
URL query parameter syntax, eg:

    /api/v5/report/{dashboardUID}?apitoken=12345&var-host=devbox

**Time span**: The time span query parameter syntax is the same as used by Grafana.
When you create a link from Grafana, you can enable the _Time range_ forwarding check-box.
The link will render a dashboard with your current time range.

**variables**: The template variable query parameter syntax is the same as used by Grafana.
When you create a link from Grafana, you can enable the _Variable values_ forwarding check-box.
The link will render a dashboard with your current variable values.

**apitoken**: A Grafana authentication api token. Use this if you have auth enabled on Grafana. Syntax: `apitoken={your-tokenstring}`.
