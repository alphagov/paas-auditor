package eventcollector

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/alphagov/paas-auditor/db"
	"github.com/alphagov/paas-auditor/eventfetchers"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

type state string

const (
	// Syncing state means that the collector has just started and need to immediately do a fetch to catchup
	Syncing state = "sync"
	// Scheduled means that we have caught up with latest events and are scheduled to run again later in Schedule time
	Scheduled state = "waiting"
)

type CfAuditEventCollector struct {
	state           state
	waitTime        time.Duration
	lastRun         time.Time
	logger          lager.Logger
	fetcher         *eventfetchers.CFAuditEventFetcher
	store           *db.EventStore
	eventsCollected int
}

func NewCfAuditEventCollector(schedule, minWaitTime, initialWaitTime time.Duration, lastRun time.Time, logger lager.Logger, fetcher *eventfetchers.CFAuditEventFetcher, store *db.EventStore) *CfAuditEventCollector {
	return &CfAuditEventCollector{
		schedule:        schedule,
		minWaitTime:     minWaitTime,
		initialWaitTime: initialWaitTime,
		lastRun:         lastRun,
		logger:          logger.Session("cf-audit-event-collector"),
		fetcher:         fetcher,
		store:           store,
		state:           Syncing,
	}
}

// Run executes collect periodically the rate is dictated by Schedule and MinWaitTime
func (c *CfAuditEventCollector) Run(ctx context.Context) {
	c.logger.Info("started")
	defer c.logger.Info("stopping")

	for {
		c.logger.Info("collecting")

		startTime := time.Now()
		collectedEventsCount, err := c.collect(ctx)
		if err != nil {
			c.logger.Error("collected.error", err)
			continue
		}
		c.eventsCollected += collectedEventsCount
		c.logger.Info("collected", lager.Data{
			"count":    collectedEventsCount,
			"duration": time.Since(startTime).String(),
		})

		select {
		case <-time.After(c.waitTime):
			continue
		case <-ctx.Done():
			c.logger.Error("context.done", err)
			return
		}
	}
}

// collect reads a batch of RawEvents from the EventFetcher and writes them to the EventStore
func (c *CfAuditEventCollector) collect(ctx context.Context) (int, error) {
	c.logger.Info("collect.start")
	// Pull an extra 5 seconds of events, to ensure we don't miss any
	pullEventsSince := c.lastRun.Add(-5 * time.Second)
	eventsChan := make(chan []cfclient.Event)
	errChan := make(chan error)
	go c.fetcher.FetchEvents(ctx, pullEventsSince, eventsChan, errChan)

	eventsCount := 0
chanloop:
	for {
		select {
		case events, stillOpen := <-eventsChan:
			if !stillOpen {
				c.logger.Info("collect.eventschan-closed")
				break chanloop
			}
			eventsCount += len(events)
			err := c.store.StoreCfAuditEvents(events)
			if err != nil {
				c.logger.Info("collect.store-error")
				return eventsCount, err
			}
			c.logger.Info("collected.page")
		case err, stillOpen := <-errChan:
			if !stillOpen {
				c.logger.Info("collect.errchan-closed")
				break chanloop
			}
			return eventsCount, err
		}
	}

	c.lastRun = time.Now()
	c.state = Scheduled
	return eventsCount, nil
}
