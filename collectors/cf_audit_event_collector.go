package eventcollector

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/alphagov/paas-auditor/db"
	"github.com/alphagov/paas-auditor/fetchers"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

type CfAuditEventCollector struct {
	waitTime        time.Duration
	logger          lager.Logger
	fetcherCfg      *fetchers.FetcherConfig
	eventDB         *db.EventStore
	eventsCollected int
}

func NewCfAuditEventCollector(waitTime time.Duration, logger lager.Logger, fetcherCfg *fetchers.FetcherConfig, eventDB *db.EventStore) *CfAuditEventCollector {
	logger = logger.Session("cf-audit-event-collector")
	return &CfAuditEventCollector{waitTime, logger, fetcherCfg, eventDB}
}

// Run executes collect periodically the rate is dictated by Schedule and MinWaitTime
func (c *CfAuditEventCollector) Run(ctx context.Context) {
	for {
		c.logger.Info("collect.start")
		pullEventsSince, err := c.pullEventsSince(5 * time.Second)
		if err != nil {
			c.logger.Error("collect.fetch.err", err)
			return err
		}

		c.logger.Info("collect.fetch", lager.Data{"pull_events_since": pullEventsSince})
		startTime := time.Now()
		err = c.collect(ctx, pullEventsSince)
		if err != nil {
			c.logger.Error("collect.fetch.err", err)
			return err
		}

		c.logger.Info("collect.done", lager.Data{"fetch_duration": time.Since(startTime)})
		select {
		case <-time.After(c.waitTime):
			continue
		case <-ctx.Done():
			c.logger.Info("context.done")
			return
		}
	}
}

func (c *CfAuditEventCollector) pullEventsSince(overlapBy time.Duration) (time.Time, error) {
	latestCFEventTime, err := c.eventDB.GetLatestCfEventTime()
	if err != nil {
		return nil, err
	}

	var pullEventsSince time.Time
	if latestCFEventTime != nil {
		// Pull an extra 5 seconds of events, to ensure we don't miss any
		pullEventsSince = latestCFEventTime.Add(-overlapBy)
	}
	return pullEventsSince, nil
}

func (c *CfAuditEventCollector) collect(ctx context.Context, pullEventsSince time.Time) error {
	c.logger.Info("collect.start")

	resultsChan := make(chan fetchers.CFAuditEventResult, 3)
	go fetchers.FetchCFAuditEvents(fetchCfg, pullEventsSince, resultsChan)

	for {
		events, err := collectOneResult(resultsChan)
		if err != nil {
			return err
		}
		if events == nil {
			return nil
		}
		err := c.eventDB.StoreCfAuditEvents(events)
		if err != nil {
			return err
		}
		c.eventsCollected += len(events)
	}
}

func collectOneResult(resultsChan chan fetchers.CFAuditEventResult) ([]cfclient.Event, error) {
	select {
	case result, stillOpen := <-resultsChan:
		if !stillOpen {
			return nil, nil
		}
		if result.Err != nil {
			return nil, result.Err
		}
		return result.Events, nil
	case <-ctx.Done():
		return nil, nil
	}
}
