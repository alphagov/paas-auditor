module github.com/alphagov/paas-auditor

go 1.12

require (
	code.cloudfoundry.org/lager v0.0.0-20180322215153-25ee72f227fe
	github.com/cloudfoundry-community/go-cfclient v0.0.0-20190611131856-16c98753d315
	github.com/cloudfoundry/gofileutils v0.0.0-20170111115228-4d0c80011a0f // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/labstack/echo v3.3.4+incompatible // indirect
	github.com/labstack/gommon v0.0.0-20180312174116-6fe1405d73ec // indirect
	github.com/lib/pq v0.0.0-20180327071824-d34b9ff171c2
	github.com/mattn/go-colorable v0.0.9 // indirect
	github.com/mattn/go-isatty v0.0.3 // indirect
	github.com/onsi/ginkgo v1.6.0
	github.com/onsi/gomega v1.4.3
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v0.0.0-20170224212429-dcecefd839c4 // indirect
)

replace github.com/cloudfoundry-community/go-cfclient => github.com/alphagov/paas-go-cfclient v0.0.0-20191004115637-b7491d6ab291
