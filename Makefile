DATABASE_URL ?= postgres://postgres:@localhost:5432/?sslmode=disable
TEST_DATABASE_URL ?= postgres://postgres:@localhost:5432/?sslmode=disable
CF_API_ADDRESS ?= $(shell cf target | awk '/api endpoint/ {print $$3}')
APP_ROOT ?= $(PWD)

bin/paas-auditor: clean
	go build -o $@ .

run-dev: bin/paas-auditor run-dev-exports
	./bin/paas-auditor

run-dev-exports:
	## Runs the application with local credentials
	$(eval export CF_API_ADDRESS=${CF_API_ADDRESS})
	$(eval export CF_CLIENT_ID=paas-auditor)
	$(eval export CF_CLIENT_SECRET=$(shell aws s3 cp s3://gds-paas-${DEPLOY_ENV}-state/cf-vars-store.yml - | awk '/uaa_clients_paas_auditor_secret/ { print $$2 }'))
	$(eval export CF_CLIENT_REDIRECT_URL=http://localhost:8881/oauth/callback)
	$(eval export CF_SKIP_SSL_VALIDATION=true)
	$(eval export DATABASE_URL=${DATABASE_URL})
	$(eval export APP_ROOT=${APP_ROOT})
	@true

clean:
	rm -f bin/paas-auditor

start_postgres_docker:
	docker run --rm -p 5432:5432 --name postgres -e POSTGRES_PASSWORD= -d postgres:9.5

stop_postgres_docker:
	docker stop postgres
	docker rm postgres
