package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alphagov/paas-auditor/fetchers"
	"github.com/alphagov/paas-auditor/pkg/collectors"
	"github.com/alphagov/paas-auditor/pkg/db"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

func main() {
	ctx, shutdown := context.WithCancel(context.Background())
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Reset(syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		shutdown()
	}()

	cfg := NewConfigFromEnv()

	pq, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		cfg.Logger.Fatal("failed to connect to database", err)
	}
	eventDB := db.NewEventStore(ctx, pq, cfg.Logger)
	if err := eventDB.Init(); err != nil {
		cfg.Logger.Fatal("failed to initialise database", err)
	}

	cfClient, err := cfclient.NewClient(cfg.CFClientConfig)
	if err != nil {
		cfg.Logger.Fatal("failed to create CF client", err)
	}

	fetcherCfg := fetchers.FetcherConfig{
		CFClient:           cfClient,
		Logger:             cfg.Logger.Session("cf-audit-event-fetcher"),
		PaginationWaitTime: cfg.PaginationWaitTime,
	}
	fetcher := func(pullEventsSince time.Time, resultsChan chan fetchers.CFAuditEventResult) {
		fetchers.FetchCFAuditEvents(&fetcherCfg, pullEventsSince, resultsChan)
	}
	collector := collectors.NewCfAuditEventCollector(cfg.Schedule, cfg.Logger, fetcher, eventDB)
	collector.Run(ctx)
}
