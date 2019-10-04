package fetchers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"code.cloudfoundry.org/lager"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

func FetchCFAuditEvents(cfg *FetcherConfig, pullEventsSince time.Time, results chan CFAuditEventResult) {
	fetchEvents(cfg, startPageURL(pullEventsSince), results)
}

type CFAuditEventResult struct {
	Events []cfclient.Event
	Err    error
}

func startPageURL(pullEventsSince time.Time) string {
	timestamp := fmt.Sprintf("timestamp>%s", pullEventsSince.Format("2006-01-02T15:04:05Z"))
	q := url.Values{}
	q.Set("q", timestamp)
	q.Set("results-per-page", "100")
	return fmt.Sprintf("/v2/events?%s", q.Encode())
}

func fetchEvents(cfg *FetcherConfig, startPageURL string, results chan CFAuditEventResult) {
	defer close(results)

	logger := cfg.Logger.WithData(lager.Data{"start_page_url": startPageURL})
	logger.Info("fetching")

	nextPageURL := startPageURL
	var events []cfclient.Event
	var err error

	for nextPageURL != "" {
		logger = logger.WithData(lager.Data{"page_url": nextPageURL})

		nextPageURL, events, err = getPage(cfg.CFClient, nextPageURL)
		if err != nil {
			logger.Error("fetched.page.error", err)
			results <- CFAuditEventResult{Err: err}
			return
		}
		logger.Info("fetched.page.ok", lager.Data{"event_count": len(events)})
		results <- CFAuditEventResult{Events: events}

		time.Sleep(cfg.PaginationWaitTime)
	}
}

func getPage(cfClient cfclient.CloudFoundryClient, url string) (string, []cfclient.Event, error) {
	resp, err := cfClient.DoRequest(cfClient.NewRequest("GET", url))
	if err != nil {
		return "", nil, fmt.Errorf("error requesting events: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	var eventResp cfclient.EventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventResp); err != nil {
		return "", nil, fmt.Errorf("error unmarshaling events: %s", err)
	}

	events := make([]cfclient.Event, len(eventResp.Resources))
	for i, e := range eventResp.Resources {
		e.Entity.GUID = e.Meta.Guid
		e.Entity.CreatedAt = e.Meta.CreatedAt
		events[i] = e.Entity
	}

	return eventResp.NextURL, events, nil
}
