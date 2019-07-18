# paas-auditor

## Overview

A Golang application that scrapes Cloud Controller's `/v2/events` endpoint for Audit Events and stores them in a Postgres database.

**This is not supported by the GOV.UK PaaS team. It is an experiment by @46bit and not yet mature.**

## Installation

You will need:

* `Go v1.12+`

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

**Note**: in development you can use `CF_USERNAME` and `CF_PASSWORD` instead of `CF_CLIENT_ID` `CF_CLIENT_SECRET` to allow it to log into Cloud Foundry
