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

func (e *CFAuditEventFetcher) FetchEvents(ctx context.Context, pullEventsSince time.Time, eventsChan chan []cfclient.Event, errChan chan error) {
	e.logger.Info("fetching", lager.Data{
		"pull_events_since": pullEventsSince,
	})

	timestamp := fmt.Sprintf("timestamp>%s", pullEventsSince.Format("2006-01-02T15:04:05Z"))
	q := url.Values{}
	q.Set("q", timestamp)
	q.Set("results-per-page", "100")
	requestURL := fmt.Sprintf("/v2/events?%s", q.Encode())

	for {
		events, nextURL, err := e.fetchEventsPage(ctx, requestURL)
		if err != nil {
			errChan <- err
			break
		}
		e.logger.Info("fetched.page", lager.Data{
			"pull_events_since": pullEventsSince,
			"page_url":          requestURL,
			"event_count":       len(events),
		})

		eventsChan <- events

		if nextURL == "" {
			break
		}
		requestURL = nextURL

		time.Sleep(e.client.Config.PaginationDelay)
	}

	close(errChan)
	close(eventsChan)
}

func (e *CFAuditEventFetcher) fetchEventsPage(ctx context.Context, requestURL string) ([]cfclient.Event, string, error) {
	r := e.client.NewRequest("GET", requestURL)
	resp, err := e.client.DoRequest(r)
	if err != nil {
		return nil, "", fmt.Errorf("error requesting events: %s", err)
	}
	defer resp.Body.Close()

	var eventResp cfclient.EventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventResp); err != nil {
		return nil, "", fmt.Errorf("error unmarshaling events: %s", err)
	}
	var events []cfclient.Event
	for _, e := range eventResp.Resources {
		e.Entity.GUID = e.Meta.Guid
		e.Entity.CreatedAt = e.Meta.CreatedAt
		events = append(events, e.Entity)
	}
	return events, eventResp.NextURL, nil
}
