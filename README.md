# paas-auditor

🎵 [`paas-billing` 2: Auditor Boogaloo](https://www.youtube.com/watch?v=4Oy7krobW78) 🎵

## Overview

A Golang application that scrapes Cloud Controller's `/v2/events` endpoint for Audit Events and stores them in a Postgres database.

**To understand how to run this and solve issues, see the [RUNBOOK](RUNBOOK.md).**

## Installation

You will need:

* `Go v1.18`

To build the application run the default make target:

```
make
```

You should then get a binary in `bin/paas-auditor`.

## Configuration

`paas-auditor` takes the following environment variables:

| Variable name | Type | Required | Default | Description |
|---|---|---|---|---|
|`APP_ROOT`|string|no|`$PWD`|absolute path to the application source to discover assets at runtime|
|`DATABASE_URL`|string|yes||Postgres connection string|
|`CF_API_ADDRESS`|string|yes||Cloud Foundry API endpoint|
|`CF_CLIENT_ID`|string|yes|| Cloud Foundry client id|
|`CF_CLIENT_SECRET`|string|yes||Cloud Foundry client secret|
|`SPLUNK_API_KEY`|string|no||Optional API key for Splunk, if provided it will send events to Splunk HEC|
|`SPLUNK_HEC_ENDPOINT_URL`|string|no||Optional URL for Splunk, if provided it will send events to Splunk HEC|
|`DEPLOY_ENV`|string|no||populates the `source` field in Splunk|
|`PORT_ENV`|string|no||port on which to listen, to serve metrics|

**Note**: in development you can use `CF_USERNAME` and `CF_PASSWORD` instead of `CF_CLIENT_ID` `CF_CLIENT_SECRET` to allow it to log into Cloud Foundry

## Metrics

`paas-auditor` exposes the following metrics via `/metrics`:

| Metric | Description |
|---|---|
|`cf_audit_event_collector_collect_duration_total`| Number of seconds spent collecting events by CF Audit Event Collector |
|`cf_audit_event_collector_errors_total`| Number of errors encountered by CF Audit Event Collector |
|`cf_audit_event_collector_events_collected_total`| Number of events collected and saved to the DB by CF Audit Event Collector |
|`cf_audit_events_to_splunk_shipper_errors_total`| Number of errors encountered by CF Audit Events to Splunk shipper |
|`cf_audit_events_to_splunk_shipper_events_shipped_total`| Number of CF audit events shipped to Splunk by CF Audit Events to Splunk shipper |
|`cf_audit_events_to_splunk_shipper_latest_event_timestamp`| Unix epoch seconds of most recent event shipped to Splunk |
|`cf_audit_events_to_splunk_shipper_ship_duration_total`| Number of seconds spent shipping events by CF Audit Events to Splunk Shipper |
|`informer_cf_audit_events_total`| Number of CF audit events in the database (This number is approximate, and depends on Postgres `reltuples`) |
|`informer_latest_cf_audit_event_timestamp`| Unix epoch seconds of most recent event in the database |

The default Go and Prometheus metrics are also exposed.
