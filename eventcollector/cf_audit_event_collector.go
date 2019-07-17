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
	// Collecting means the collector thinks it probably has more to collect but is rate limited by MinWaitTime
	Collecting state = "collecting"
)

type CfAuditEventCollector struct {
	state           state
	schedule        time.Duration
	minWaitTime     time.Duration
	initialWaitTime time.Duration
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
func (c *CfAuditEventCollector) Run(ctx context.Context) error {
	c.logger.Info("started")
	defer c.logger.Info("stopping")

	for {
		c.logger.Info("status", lager.Data{
			"state":            c.state,
			"next_collection":  c.waitDuration().String(),
			"events_collected": c.eventsCollected,
		})
		select {
		case <-time.After(c.waitDuration()):
			startTime := time.Now()
			collectedEventsCount, err := c.collect(ctx)
			if err != nil {
				c.state = Scheduled
				c.logger.Error("collect-error", err)
				continue
			}
			elapsedTime := time.Since(startTime)
			c.eventsCollected += collectedEventsCount
			c.logger.Info("collected", lager.Data{
				"count":    collectedEventsCount,
				"duration": elapsedTime.String(),
			})
		// To be able to exit cleanly
		case <-ctx.Done():
			return nil
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
	for {
		select {
		case events, stillOpen := <-eventsChan:
			if !stillOpen {
				c.logger.Info("collect.eventschan-closed")
				break
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
				break
			}
			return eventsCount, err
		}
	}

	c.lastRun = time.Now()
	c.state = Scheduled
	return eventsCount, nil
}

// wait returns a channel that closes after the collection schedule time has elapsed
func (c *CfAuditEventCollector) waitDuration() time.Duration {
	delay := c.schedule
	if c.state == Syncing {
		delay = c.initialWaitTime
	}
	if c.state == Collecting {
		delay = c.minWaitTime
	}
	return delay
}
