package eventfetchers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"code.cloudfoundry.org/lager"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

type CFAuditEventResult {
	Events: []cfclient.Event,
	Err: error
}

type CFAuditEventFetcher struct {
	client             *cfclient.Client
	logger             lager.Logger
	paginationWaitTime time.Duration
}

func NewCFAuditEventFetcher(client *cfclient.Client, logger lager.Logger, paginationWaitTime time.Duration) *CFAuditEventFetcher {
	return &CFAuditEventFetcher{
		client:             client,
		logger:             logger.Session("cf-audit-event-fetcher"),
		paginationWaitTime: paginationWaitTime,
	}
}

func (e *CFAuditEventFetcher) FetchEvents(pullEventsSince time.Time, results chan CFAuditEventResult) {
	logger := e.logger.WithData(lager.Data{"pull_events_since": pullEventsSince})
	logger.Info("fetching")
	nextPageURL := startPageURL(pullEventsSince)

	defer close(results)

	for nextPageURL != "" {
		logger = logger.WithData(lager.Data{"page_url": nextPageURL})

		nextPageURL, events, err := e.getPage(nextPageURL, e.client)
		if err != nil {
			logger.Error("fetched.page.error", err)
			results <- CFAuditEventResult{Err: err}
			return
		}
		logger.Info("fetched.page.ok", lager.Data{"event_count": len(events)})
		results <- CFAuditEventResult{Events: events}

		time.Sleep(e.paginationWaitTime)
	}
}

func startPageURL(pullEventsSince time.Time) string {
	timestamp := fmt.Sprintf("timestamp>%s", pullEventsSince.Format("2006-01-02T15:04:05Z"))
	q := url.Values{}
	q.Set("q", timestamp)
	q.Set("results-per-page", "100")
	return fmt.Sprintf("/v2/events?%s", q.Encode())
}

func getPage(url string, client *cfclient.Client) (string, []cfclient.Event, error) {
	resp, err := e.client.DoRequest(e.client.NewRequest("GET", url))
	if err != nil {
		return "", nil, fmt.Errorf("error requesting events: %s", err)
	}
	defer resp.Body.Close()

	var eventResp cfclient.EventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventResp); err != nil {
		return "", nil, fmt.Errorf("error unmarshaling events: %s", err)
	}

	events := make([]cfclient.Event, len(eventResp.Resources))
	for _, e := range eventResp.Resources {
		e.Entity.GUID = e.Meta.Guid
		e.Entity.CreatedAt = e.Meta.CreatedAt
		events = append(events, e.Entity)
	}

	return eventResp.NextURL, events, nil
}
