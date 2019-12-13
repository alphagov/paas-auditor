# Runbook for `paas-auditor`

## How to deploy

For GOV.UK PaaS, the `create-cloudfoundry` pipeline deploys paas-auditor.

For other CF deployments you'll need three things:

* For your `cf` CLI to be logged into Cloud Foundry;
* For the `DEPLOY_ENV` environment variable to be set appropriately;
* To have already created a Postgres service named `auditor-db`.

```
cd paas-auditor

cf push \
   --no-start \
   --var cf_api_address="$CF_API_ADDRESS" \
   --var cf_client_id="$CF_CLIENT_ID" \
   --var cf_client_secret="$CF_CLIENT_SECRET" \
   --var deploy_env="$DEPLOY_ENV" \
   --var splunk_api_key="$SPLUNK_API_KEY" \
   --var splunk_hec_endpoint_url="$SPLUNK_HEC_ENDPOINT_URL"

cf bind-service paas-auditor auditor-db

cf start paas-auditor
```

## What it does

A few seconds after starting up, `paas-auditor` will first fetch audit events from Cloud Controller's `/v2/events` endpoint. How much data it fetches depends on whether the database already has events stored:

* If your database is empty it will fetch the last 4 weeks of data. potentially tens of thousands of pages (100 events per page.)
* If your database already has events stored, it will fetch data since the most recent event. To ensure nothing is missed, it actually fetches data from 5 minutes before then.

Once that initial fetching finishes, it will wake up to bring itself up to date every few minutes.

If `SPLUNK_API_KEY` and `SPLUNK_HEC_ENDPOINT_URL` environment variables are
sent then paas-auditor will also ship audit events to Splunk.

### How to observe it working

You should be able to see quite descriptive logs from:

```
cf logs paas-auditor
```

You can also connect to the database to understand its state:

```
cf conduit auditor-db -- psql
```

Here's a useful query to show the number of events stored and how up-to-date they are:

```
SELECT COUNT(*), MAX(created_at) FROM cf_audit_events;
```

The application also exposes metrics via Prometheus exposition format, accessible via `/metrics`

## Dealing with issues

### It's OK to stop it

Cloud Controller stores Audit Events for about 31 days. If Cloud Controller is experiencing high load you are absolutely fine to stop it.

### How to stop it

It's typically in the `admin` org's `billing` space. Straightforwardly `cf stop` the app:

```
cf stop paas-auditor
```

All requests to Cloud Controller should stop within seconds.
