package collectors

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/alphagov/paas-auditor/pkg/db"
	"github.com/alphagov/paas-auditor/pkg/fetchers"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

type CfAuditEventCollector struct {
	schedule        time.Duration
	logger          lager.Logger
	fetcher         fetchers.CFAuditEventFetcher
	eventDB         db.EventDB
	eventsCollected int
}

func NewCfAuditEventCollector(
	schedule time.Duration,
	logger lager.Logger,
	fetcher fetchers.CFAuditEventFetcher,
	eventDB db.EventDB,
) *CfAuditEventCollector {
	logger = logger.Session("cf-audit-event-collector")
	return &CfAuditEventCollector{schedule, logger, fetcher, eventDB, 0}
}

func (c *CfAuditEventCollector) Run(ctx context.Context) error {
	lsession := c.logger.Session("run")
	lsession.Info("start")
	defer lsession.Info("end")

	for {
		pullEventsSince, err := c.pullEventsSince(5 * time.Second)
		if err != nil {
			lsession.Error("pull-events-since", err)
			return err
		}

		select {
		case <-ctx.Done():
			lsession.Info("done")
			return nil
		case <-time.After(c.schedule):
			continue
		default:
			startTime := time.Now()

			resultsChan := make(chan fetchers.CFAuditEventResult, 3)
			go c.fetcher(pullEventsSince, resultsChan)

			var events []cfclient.Event

			for result := range resultsChan {
				if result.Err != nil {
					lsession.Error("err-recv-events", err)
					return result.Err
				}

				events = append(events, result.Events...)
			}

			err := c.eventDB.StoreCfAuditEvents(events)
			if err != nil {
				lsession.Error("err-store-cf-audit-events", err)
				return err
			}
			c.eventsCollected += len(events)

			lsession.Info(
				"stored-events",
				lager.Data{
					"duration":         time.Since(startTime),
					"events-collected": c.eventsCollected,
				},
			)
		}
	}
}

func (c *CfAuditEventCollector) pullEventsSince(overlapBy time.Duration) (time.Time, error) {
	latestCFEventTime, err := c.eventDB.GetLatestCfEventTime()

	if err != nil {
		return time.Time{}, err
	}

	if latestCFEventTime == nil {
		return time.Time{}, nil
	}

	return (*latestCFEventTime).Add(-overlapBy), nil
}
