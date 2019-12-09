package collectors

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/alphagov/paas-auditor/fetchers"
	"github.com/alphagov/paas-auditor/pkg/db"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

type CfAuditEventCollector struct {
	schedule        time.Duration
	logger          lager.Logger
	fetcher         fetchers.CFAuditEventFetcher
	eventDB         *db.EventStore
	eventsCollected int
}

func NewCfAuditEventCollector(schedule time.Duration, logger lager.Logger, fetcher fetchers.CFAuditEventFetcher, eventDB *db.EventStore) *CfAuditEventCollector {
	logger = logger.Session("cf-audit-event-collector")
	return &CfAuditEventCollector{schedule, logger, fetcher, eventDB, 0}
}

func (c *CfAuditEventCollector) Run(ctx context.Context) error {
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
		case <-time.After(c.schedule):
			continue
		case <-ctx.Done():
			c.logger.Info("context.done")
			return nil
		}
	}
}

func (c *CfAuditEventCollector) pullEventsSince(overlapBy time.Duration) (time.Time, error) {
	// latestCFEventTime, err := c.eventDB.GetLatestCfEventTime()
	// if err != nil {
	// 	return time.Now(), err
	// }

	// var pullEventsSince time.Time
	// if latestCFEventTime != nil {
	// 	pullEventsSince = (*latestCFEventTime).Add(-overlapBy)
	// }
	return time.Now(), fmt.Errorf("Unimplemented") // TODO(paroxp, tlwr)
}

func (c *CfAuditEventCollector) collect(ctx context.Context, pullEventsSince time.Time) error {
	c.logger.Info("collect.start")

	resultsChan := make(chan fetchers.CFAuditEventResult, 3)
	go c.fetcher(pullEventsSince, resultsChan)

	for {
		var events []cfclient.Event

		select {
		case <-ctx.Done():
			return nil
		case result, stillOpen := <-resultsChan:
			if !stillOpen {
				return nil
			}
			if result.Err != nil {
				return result.Err
			}
			events = result.Events
		}

		err := c.eventDB.StoreCfAuditEvents(events)
		if err != nil {
			return err
		}
		c.eventsCollected += len(events)
	}
}
