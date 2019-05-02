package eventfetchers

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"code.cloudfoundry.org/lager"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

type CFAuditEventFetcher struct {
	client *cfclient.Client
	logger lager.Logger
}

func NewCFAuditEventFetcher(client *cfclient.Client, logger lager.Logger) *CFAuditEventFetcher {
	return &CFAuditEventFetcher{
		client: client,
		logger: logger.Session("cf-audit-event-fetcher"),
	}
}

func (e *CFAuditEventFetcher) FetchEvents(ctx context.Context, lastRun time.Time) ([]cfclient.Event, error) {
	e.logger.Info("fetching", lager.Data{
		"last_run": lastRun,
	})

	timestamp := fmt.Sprintf("timestamp>%s", lastRun.Format("2006-01-02T15:04:05Z"))
	q := url.Values{}
	q.Set("q", timestamp)

	events, err := e.client.ListEventsByQuery(q)
	if err != nil {
		return nil, err
	}

	e.logger.Info("fetched", lager.Data{
		"last_run":    lastRun,
		"event_count": len(events),
	})

	return events, nil
}
