package main

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	cfclient "github.com/cloudfoundry-community/go-cfclient"

	"code.cloudfoundry.org/lager"
)

type Config struct {
	DeployEnv string

	Logger      lager.Logger
	DatabaseURL string

	CFClientConfig *cfclient.Config

	PaginationWaitTime time.Duration
	CollectorSchedule  time.Duration
	ShipperSchedule    time.Duration

	SplunkAPIKey string
	SplunkURL    string

	ListenPort uint
}

func NewConfigFromEnv() Config {
	return Config{
		DeployEnv: getEnvWithDefaultString("DEPLOY_ENV", "dev"),

		Logger:      getDefaultLogger(),
		DatabaseURL: getEnvWithDefaultString("DATABASE_URL", "postgres://postgres:@localhost:5432/"),

		CFClientConfig: &cfclient.Config{
			ApiAddress:        os.Getenv("CF_API_ADDRESS"),
			Username:          os.Getenv("CF_USERNAME"),
			Password:          os.Getenv("CF_PASSWORD"),
			ClientID:          os.Getenv("CF_CLIENT_ID"),
			ClientSecret:      os.Getenv("CF_CLIENT_SECRET"),
			SkipSslValidation: os.Getenv("CF_SKIP_SSL_VALIDATION") == "true",
			Token:             os.Getenv("CF_TOKEN"),
			UserAgent:         os.Getenv("CF_USER_AGENT"),
			HttpClient: &http.Client{
				Timeout: 30 * time.Second,
			},
		},

		PaginationWaitTime: getEnvWithDefaultDuration("FETCHER_PAGINATION_WAIT_TIME", 200*time.Millisecond),
		CollectorSchedule:  getEnvWithDefaultDuration("COLLECTOR_SCHEDULE", 2*time.Minute),
		ShipperSchedule:    getEnvWithDefaultDuration("SHIPPER_SCHEDULE", 15*time.Second),

		SplunkAPIKey: os.Getenv("SPLUNK_API_KEY"),
		SplunkURL:    os.Getenv("SPLUNK_HEC_ENDPOINT_URL"),

		ListenPort: getEnvWithDefaultInt("PORT", 9299),
	}
}

func getEnvWithDefaultDuration(k string, def time.Duration) time.Duration {
	v := getEnvWithDefaultString(k, "")
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		panic(err)
	}
	return d
}

func getEnvWithDefaultString(k string, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func getEnvWithDefaultInt(k string, def uint) uint {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		panic(err)
	}
	return uint(d)
}

func getDefaultLogger() lager.Logger {
	logger := lager.NewLogger("paas-auditor")
	logLevel := lager.INFO
	if strings.ToLower(os.Getenv("LOG_LEVEL")) == "debug" {
		logLevel = lager.DEBUG
	}
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, logLevel))

	return logger
}
