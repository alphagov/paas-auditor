package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/alphagov/paas-auditor/pkg/collectors"
	"github.com/alphagov/paas-auditor/pkg/db"
	"github.com/alphagov/paas-auditor/pkg/fetchers"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.PrometheusListenPort),
		Handler: mux,
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		err := collector.Run(ctx)
		if err != nil {
			cfg.Logger.Error("Collector encountered error: %s", err)
		}
		shutdown()
		os.Exit(1)
	}()

	wg.Add(1)
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			cfg.Logger.Error("Server encountered error: %s", err)
		}
		shutdown()
		os.Exit(1)
	}()

	wg.Wait()
}
