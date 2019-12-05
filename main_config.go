package main

import (
	"net/http"
	"os"
	"strings"
	"time"

	cfclient "github.com/cloudfoundry-community/go-cfclient"

	"code.cloudfoundry.org/lager"
)

type Config struct {
	Logger             lager.Logger
	DatabaseURL        string
	CFClientConfig     *cfclient.Config
	Schedule           time.Duration
	PaginationWaitTime time.Duration
}

func NewConfigFromEnv() Config {
	return Config{
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
		Schedule:           getEnvWithDefaultDuration("SCHEDULE", 5*time.Minute),
		PaginationWaitTime: getEnvWithDefaultDuration("FETCHER_PAGINATION_WAIT_TIME", 200*time.Millisecond),
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

func getDefaultLogger() lager.Logger {
	logger := lager.NewLogger("paas-auditor")
	logLevel := lager.INFO
	if strings.ToLower(os.Getenv("LOG_LEVEL")) == "debug" {
		logLevel = lager.DEBUG
	}
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, logLevel))

	return logger
}
