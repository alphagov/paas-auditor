package collectors

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/alphagov/paas-auditor/pkg/db"
	"github.com/alphagov/paas-auditor/pkg/fetchers"
)

type CFAuditEventCollector struct {
	schedule        time.Duration
	logger          lager.Logger
	fetcher         fetchers.CFAuditEventFetcher
	eventDB         db.EventDB
	eventsCollected int
}

func NewCFAuditEventCollector(
	schedule time.Duration,
	logger lager.Logger,
	fetcher fetchers.CFAuditEventFetcher,
	eventDB db.EventDB,
) *CFAuditEventCollector {
	logger = logger.Session("cf-audit-event-collector")
	return &CFAuditEventCollector{schedule, logger, fetcher, eventDB, 0}
}

func (c *CFAuditEventCollector) Run(ctx context.Context) error {
	lsession := c.logger.Session("run")
	lsession.Info("start")
	defer lsession.Info("end")

	for {
		pullEventsSince, err := c.pullEventsSince(5 * time.Second)
		if err != nil {
			lsession.Error("err-pull-events-since", err)
			CFAuditEventCollectorErrorsTotal.Inc()
			return err
		}

		select {
		case <-ctx.Done():
			lsession.Info("done")
			return nil
		case <-time.After(c.schedule):
			startTime := time.Now()

			resultsChan := make(chan fetchers.CFAuditEventResult, 3)
			go c.fetcher(pullEventsSince, resultsChan)

			for result := range resultsChan {
				if result.Err != nil {
					lsession.Error("err-recv-events", err)
					CFAuditEventCollectorErrorsTotal.Inc()
					return result.Err
				}

				err := c.eventDB.StoreCFAuditEvents(result.Events)
				if err != nil {
					lsession.Error("err-store-cf-audit-events", err)
					CFAuditEventCollectorErrorsTotal.Inc()
					return err
				}

				c.eventsCollected += len(result.Events)
				CFAuditEventCollectorEventsCollectedTotal.Add(float64(len(result.Events)))

				lsession.Info(
					"stored-events",
					lager.Data{
						"duration":         time.Since(startTime),
						"events-collected": c.eventsCollected,
					},
				)
			}

			duration := time.Since(startTime)
			lsession.Info(
				"stored-all-events",
				lager.Data{
					"duration":         duration,
					"events-collected": c.eventsCollected,
				},
			)
			CFAuditEventCollectorEventsCollectDurationTotal.Add(duration.Seconds())
		}
	}
}

func (c *CFAuditEventCollector) pullEventsSince(overlapBy time.Duration) (time.Time, error) {
	latestCFEventTime, err := c.eventDB.GetLatestCFEventTime()

	if err != nil {
		return latestCFEventTime, err
	}

	startTime := latestCFEventTime.Add(-overlapBy)
	if startTime.Year() < 1970 {
		return latestCFEventTime, nil
	}
	return startTime, nil
}
