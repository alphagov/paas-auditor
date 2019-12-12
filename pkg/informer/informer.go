package informer

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/alphagov/paas-auditor/pkg/db"
)

type Informer struct {
	schedule time.Duration
	logger   lager.Logger
	eventDB  db.EventDB
}

func NewInformer(
	schedule time.Duration,
	logger lager.Logger,
	eventDB db.EventDB,
) *Informer {
	logger = logger.Session("informer")
	return &Informer{schedule, logger, eventDB}
}

func (i *Informer) Run(ctx context.Context) error {
	lsession := i.logger.Session("run")

	lsession.Info("start")
	defer lsession.Info("end")

	for {
		select {
		case <-ctx.Done():
			lsession.Info("done")
			return nil
		case <-time.After(i.schedule):
			count, err := i.eventDB.GetCfEventCount()
			if err != nil {
				lsession.Error("err-event-db-get-cf-event-count", err)
			}
			InformerCFAuditEventsTotal.Set(float64(count)) // this will be 0 if err

			timestamp, err := i.eventDB.GetLatestCfEventTime()
			if err != nil {
				lsession.Error("err-event-db-get-latest-cf-event-time", err)
				InformerLatestCFAuditEventTimestamp.Set(float64(0))
			} else {
				InformerLatestCFAuditEventTimestamp.Set(float64(timestamp.Unix()))
			}

		}
	}
}
