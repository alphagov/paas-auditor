package eventcollector

import (
	"context"
	"sync"
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
	mu              sync.Mutex
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
	c.mu.Lock()
	defer c.mu.Unlock()

	for {
		c.logger.Info("status", lager.Data{
			"state":            c.state,
			"next_collection":  c.waitDuration().String(),
			"events_collected": c.eventsCollected,
		})
		select {
		case <-time.After(c.waitDuration()):
			startTime := time.Now()
			collectedEvents, err := c.collect(ctx)
			if err != nil {
				c.state = Scheduled
				c.logger.Error("collect-error", err)
				continue
			}
			elapsedTime := time.Since(startTime)
			c.eventsCollected += len(collectedEvents)
			c.logger.Info("collected", lager.Data{
				"count":    len(collectedEvents),
				"duration": elapsedTime.String(),
			})
		case <-ctx.Done():
			return nil
		}
	}
}

// collect reads a batch of RawEvents from the EventFetcher and writes them to the EventStore
func (c *CfAuditEventCollector) collect(ctx context.Context) ([]cfclient.Event, error) {
	events, err := c.fetcher.FetchEvents(ctx, c.lastRun)
	if err != nil {
		return nil, err
	}

	if err := c.store.StoreCfAuditEvents(events); err != nil {
		return nil, err
	}

	c.lastRun = time.Now()
	c.state = Scheduled
	return events, nil
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
