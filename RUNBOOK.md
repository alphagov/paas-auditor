# Runbook for `paas-auditor`

## How to deploy

At the time of writing it is deployed manually rather than using our pipelines. We can integrate it into our pipelines if/when its data gets actually used for anything.

The details below are GOV.UK PaaS-specific, but the general idea works for any Cloud Foundry.

To run the below steps you'll need three things:

* For your `cf` CLI to be logged into Cloud Foundry;
* For the `DEPLOY_ENV` environment variable to be set appropriately;
* For your AWS access to be configured so the `aws` CLI will work.
* To have already created a Postgres service named `auditor-db`.

```
cd paas-auditor
cf target -o admin -s billing
cf push --no-start
cf bind-service paas-auditor auditor-db

CF_API_ADDRESS=$(cf target | awk '/api endpoint:/ { print $3 }')
CF_CLIENT_SECRET=$(aws s3 cp "s3://gds-paas-${DEPLOY_ENV}-state/cf-vars-store.yml" - | awk '/uaa_clients_paas_auditor_secret/ { print $2 }')

cf set-env paas-auditor CF_API_ADDRESS "$CF_API_ADDRESS"
cf set-env paas-auditor CF_CLIENT_ID paas-auditor
cf set-env paas-auditor CF_CLIENT_SECRET "$CF_CLIENT_SECRET"

cf start paas-auditor
```

## What it does

A few seconds after starting up, `paas-auditor` will first fetch audit events from Cloud Controller's `/v2/events` endpoint. How much data it fetches depends on whether the database already has events stored:

* If your database is empty it will fetch the last 4 weeks of data. potentially tens of thousands of pages (100 events per page.)
* If your database already has events stored, it will fetch data since the most recent event. To ensure nothing is missed, it actually fetches data from 5 minutes before then.

Once that initial fetching finishes, it will wake up to bring itself up to date every 5 minutes.

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

## Dealing with issues

### It's OK to stop it

Cloud Controller stores Audit Events for about 31 days. If Cloud Controller is experiencing high load you are absolutely fine to stop it.

### How to stop it

It's typically in the `admin` org's `billing` space. Straightforwardly `cf stop` the app:

```
cf stop paas-auditor
```

All requests to Cloud Controller should stop within seconds.
