package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alphagov/paas-auditor/db"
	"github.com/alphagov/paas-auditor/eventcollector"
	"github.com/alphagov/paas-auditor/eventfetchers"
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
	if cfg.DatabaseURL == "" {
		cfg.Logger.Fatal("Store or DatabaseURL must be provided in Config", nil)
	}

	pq, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		cfg.Logger.Fatal("failed to connect to database", err)
	}
	store := db.NewEventStore(ctx, pq, cfg.Logger)
	if err := store.Init(); err != nil {
		cfg.Logger.Fatal("failed to initialise store", err)
	}

	lastRun, err := store.LastSeenEvent()
	if err != nil {
		cfg.Logger.Fatal("error finding last seen event", err)
	}
	if lastRun == nil {
		// If the database is empty, get the last week of data
		fourWeeksAgo := time.Now().AddDate(0, 0, -28)
		lastRun = &fourWeeksAgo
	}

	cf, err := cfclient.NewClient(cfg.CFClientConfig)
	if err != nil {
		cfg.Logger.Fatal("failed to create CF client", err)
	}
	fetcher := eventfetchers.NewCFAuditEventFetcher(cf, cfg.Logger, cfg.PaginationWaitTime)

	collector := eventcollector.NewCfAuditEventCollector(
		cfg.Schedule,
		cfg.MinWaitTime,
		cfg.InitialWaitTime,
		*lastRun,
		cfg.Logger,
		fetcher,
		store,
	)
	collector.Run(ctx)
}
