package shippers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	"github.com/gojektech/heimdall"
	"github.com/gojektech/heimdall/httpclient"

	"github.com/alphagov/paas-auditor/pkg/db"
)

const (
	cfAuditEventsToSplunkShipperName = "cf-audit-events-to-splunk"
)

type splunkEvent struct {
	SourceType string      `json:"sourcetype"`
	Source     string      `json:"source"`
	Event      interface{} `json:"event"`
}

type splunkHTTPClient struct {
	client       http.Client
	splunkAPIKey string
}

func (c *splunkHTTPClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", fmt.Sprintf("Splunk %s", c.splunkAPIKey))
	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

type CfAuditEventsToSplunkShipper struct {
	schedule  time.Duration
	logger    lager.Logger
	eventDB   db.EventDB
	deployEnv string
	client    *httpclient.Client
	splunkURL string

	eventsShipped int
}

func NewCfAuditEventsToSplunkShipper(
	schedule time.Duration,
	logger lager.Logger,
	eventDB db.EventDB,
	deployEnv string,
	splunkAPIKey string,
	splunkURL string,
) *CfAuditEventsToSplunkShipper {
	logger = logger.Session("cf-audit-events-to-splunk-shipper")

	var (
		requestTimeout         = 5 * time.Second
		initalTimeout          = 5 * time.Second
		maxTimeout             = 15 * time.Second
		exponent       float64 = 2
		jitter                 = 500 * time.Millisecond
		maxRetries             = 10

		backoff = heimdall.NewExponentialBackoff(
			initalTimeout, maxTimeout,
			exponent, jitter,
		)

		retrier = heimdall.NewRetrier(backoff)
	)

	client := httpclient.NewClient(
		httpclient.WithHTTPClient(&splunkHTTPClient{
			client:       *http.DefaultClient,
			splunkAPIKey: splunkAPIKey,
		}),
		httpclient.WithHTTPTimeout(requestTimeout),
		httpclient.WithRetrier(retrier),
		httpclient.WithRetryCount(maxRetries),
	)

	return &CfAuditEventsToSplunkShipper{
		schedule, logger, eventDB, deployEnv, client, splunkURL, 0,
	}
}

func (s *CfAuditEventsToSplunkShipper) Run(ctx context.Context) error {
	lsession := s.logger.Session("run")

	lsession.Info("start")
	defer lsession.Info("end")

	for {
		select {
		case <-ctx.Done():
			lsession.Info("done")
			return nil
		case <-time.After(s.schedule):
			startTime := time.Now()

			eventsToShip, err := s.eventDB.GetUnshippedCfAuditEventsForShipper(
				cfAuditEventsToSplunkShipperName,
			)

			if err != nil {
				lsession.Error("err-get-unshipped-cf-audit-events-for-shipper", err)
				CFAuditEventsToSplunkShipperErrorsTotal.Inc()
				continue
			}

			var (
				shippedEvents    = make([]cfclient.Event, 0)
				allEventsShipped = true
			)

			for _, event := range eventsToShip {
				err := s.shipEvent(event)

				if err != nil {
					lsession.Error("err-ship-event", err)
					allEventsShipped = false
					CFAuditEventsToSplunkShipperErrorsTotal.Inc()
					break
				}

				shippedEvents = append(shippedEvents, event)
				s.eventsShipped++
				CFAuditEventsToSplunkShipperEventsShippedTotal.Inc()
			}

			if len(shippedEvents) > 0 {
				lastEvent := shippedEvents[len(shippedEvents)-1]

				err := s.eventDB.UpdateShipperCursor(
					cfAuditEventsToSplunkShipperName,
					lastEvent.CreatedAt, lastEvent.GUID,
				)

				if err != nil {
					lsession.Error("err-update-shipper-cursor", err, lager.Data{
						"shipper": cfAuditEventsToSplunkShipperName,
					})
					CFAuditEventsToSplunkShipperErrorsTotal.Inc()
					continue
				}

				lastEventCreatedAt, err := time.Parse(time.RFC3339, lastEvent.CreatedAt)
				if err != nil {
					// Not fatal
					lsession.Error("err-parse-event-time", err, lager.Data{
						"raw-created-at": lastEvent.CreatedAt,
					})
					CFAuditEventsToSplunkShipperErrorsTotal.Inc()
					continue
				}
				CFAuditEventsToSplunkShipperLatestEventTimestamp.Set(
					float64(lastEventCreatedAt.Unix()),
				)
			}

			duration := time.Since(startTime)
			lsession.Info(
				"shipped-events",
				lager.Data{
					"duration":           duration,
					"events-shipped":     s.eventsShipped,
					"all-events-shipped": allEventsShipped,
				},
			)
			CFAuditEventsToSplunkShipperShipDurationTotal.Add(duration.Seconds())
		}
	}
}

func (s *CfAuditEventsToSplunkShipper) shipEvent(event cfclient.Event) error {
	bytesToShip, err := json.Marshal(splunkEvent{
		SourceType: "cf-audit-event",
		Source:     s.deployEnv,
		Event:      event,
	})

	if err != nil {
		return err
	}

	resp, err := s.client.Post(
		s.splunkURL,
		bytes.NewReader(bytesToShip),
		http.Header{},
	)

	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()

	if err != nil {
		return err
	}

	if 200 <= resp.StatusCode && resp.StatusCode < 300 {
		return nil
	}

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	return fmt.Errorf("Status: %d Body: %s", resp.StatusCode, body)
}
