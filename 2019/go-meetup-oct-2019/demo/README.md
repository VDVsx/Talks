# Demo

Demo used in the Helsinki Gophers October meetup, the demo app and other code files are for demo propose, some shortcuts were taken, **DON'T** take anything from it as reference for production code.

## Pre-conditions

* Start docker composition `docker-compose up`
* In your browser navigate to `http://localhost:3000/` and do the initial Grafana setup
* Add Loki and Prometheus as a datasource in Grafana
* Add another Prometheus datasource with name `loki-prom` with Loki url `http://loki:3100/loki`
* Import the Grafana dashboard from this repo

## Demo Script

* Show the Gopher app at `http://localhost:8000/`
* Upload a image having `Gopher` in the name preferably a Gopher :)
* Upload another file not having `Gopher` in the name, a error will be created, show it in Loki
* Show the app greeting endpoint `http://localhost:8000/hello` and make the Gopher tired by using some load testing tool, by default it has 3 stages and 3 different images per stage.
* Once you get errors show the Grafana dashboards and respective alarms, introduce the explorer workflow, show how you could annotate events, like for example a load test event.
* Show the metrics of the instrumented endpoints and the Go runtime in Grafana, same is also available in Prometheus UI at `http://localhost:9090/`

In the demo I used a browser extension to slow refresh the tab and a load testing tool written in Go called [Hey](https://github.com/rakyll/hey).

Gophers used are from [Free Gopher Pack by Maria Letta](https://github.com/MariaLetta/free-gophers-pack).
