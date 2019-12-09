package fetchers_test

import (
	"fmt"
	"github.com/satori/go.uuid"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"code.cloudfoundry.org/lager"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/alphagov/paas-auditor/pkg/fetchers"
)

const (
	cfAPIURL  = "http://cf.api"
	uaaAPIURL = "http://uaa.api"
)

func wrapEventsForResponse(
	pages int,
	nextUrl string,
	events []cfclient.Event,
) cfclient.EventsResponse {

	eventResources := make([]cfclient.EventResource, len(events))
	for i, event := range events {
		eventResources[i] = cfclient.EventResource{
			Meta: cfclient.Meta{
				Guid:      event.GUID,
				CreatedAt: event.CreatedAt,
			},
			Entity: event,
		}
	}

	return cfclient.EventsResponse{
		TotalResults: len(eventResources),
		Pages:        pages,
		NextURL:      nextUrl,
		Resources:    eventResources,
	}
}

func randomEvent() cfclient.Event {
	createdAt := time.Unix(rand.Int63(), 0)
	return cfclient.Event{
		GUID:             uuid.NewV4().String(),
		Type:             "test.event.type",
		CreatedAt:        createdAt.Format("2006-01-02T15:04:05Z"),
		Actor:            fmt.Sprintf("test-actor-"),
		ActorType:        fmt.Sprintf("test-actor-type-"),
		ActorName:        fmt.Sprintf("test-actor-name-"),
		ActorUsername:    fmt.Sprintf("test-actor-username-"),
		Actee:            fmt.Sprintf("test-actee-"),
		ActeeType:        fmt.Sprintf("test-actee-type-"),
		ActeeName:        fmt.Sprintf("test-actee-name-"),
		OrganizationGUID: uuid.NewV4().String(),
		SpaceGUID:        uuid.NewV4().String(),
		Metadata:         nil, //map[string]interface{},
	}
}

func randomEvents(n int) []cfclient.Event {
	events := make([]cfclient.Event, n)
	for i := 0; i < n; i += 1 {
		events[i] = randomEvent()
	}
	return events
}

func randomEventPages(numberOfPages, eventsPerPage int) [][]cfclient.Event {
	eventPages := make([][]cfclient.Event, numberOfPages)
	for page := 0; page < numberOfPages; page += 1 {
		eventPages[page] = randomEvents(eventsPerPage)
	}
	return eventPages
}

var _ = Describe("CFAuditEvents Fetcher", func() {
	var cfg *fetchers.FetcherConfig

	BeforeEach(func() {
		httpclient := &http.Client{Transport: &http.Transport{}}
		httpmock.ActivateNonDefault(httpclient)

		httpmock.RegisterResponder(
			"GET",
			fmt.Sprintf("%s/v2/info", cfAPIURL),
			httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{
				"token_endpoint": fmt.Sprintf("%s", uaaAPIURL),
			}),
		)

		httpmock.RegisterResponder(
			"POST",
			fmt.Sprintf("%s/oauth/token", uaaAPIURL),
			httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{
				// Copy and pasted from UAA docs
				"access_token":  "acb6803a48114d9fb4761e403c17f812",
				"token_type":    "bearer",
				"id_token":      "eyJhbGciOiJIUzI1NiIsImprdSI6Imh0dHBzOi8vbG9jYWxob3N0OjgwODAvdWFhL3Rva2VuX2tleXMiLCJraWQiOiJsZWdhY3ktdG9rZW4ta2V5IiwidHlwIjoiSldUIn0.eyJzdWIiOiIwNzYzZTM2MS02ODUwLTQ3N2ItYjk1Ny1iMmExZjU3MjczMTQiLCJhdWQiOlsibG9naW4iXSwiaXNzIjoiaHR0cDovL2xvY2FsaG9zdDo4MDgwL3VhYS9vYXV0aC90b2tlbiIsImV4cCI6MTU1NzgzMDM4NSwiaWF0IjoxNTU3Nzg3MTg1LCJhenAiOiJsb2dpbiIsInNjb3BlIjpbIm9wZW5pZCJdLCJlbWFpbCI6IndyaHBONUB0ZXN0Lm9yZyIsInppZCI6InVhYSIsIm9yaWdpbiI6InVhYSIsImp0aSI6ImFjYjY4MDNhNDgxMTRkOWZiNDc2MWU0MDNjMTdmODEyIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsImNsaWVudF9pZCI6ImxvZ2luIiwiY2lkIjoibG9naW4iLCJncmFudF90eXBlIjoiYXV0aG9yaXphdGlvbl9jb2RlIiwidXNlcl9uYW1lIjoid3JocE41QHRlc3Qub3JnIiwicmV2X3NpZyI6ImI3MjE5ZGYxIiwidXNlcl9pZCI6IjA3NjNlMzYxLTY4NTAtNDc3Yi1iOTU3LWIyYTFmNTcyNzMxNCIsImF1dGhfdGltZSI6MTU1Nzc4NzE4NX0.Fo8wZ_Zq9mwFks3LfXQ1PfJ4ugppjWvioZM6jSqAAQQ",
				"refresh_token": "f59dcb5dcbca45f981f16ce519d61486-r",
				"expires_in":    43199,
				"scope":         "openid oauth.approvals",
				"jti":           "acb6803a48114d9fb4761e403c17f812",
			}),
		)

		cfClient, err := cfclient.NewClient(&cfclient.Config{
			ApiAddress: cfAPIURL,
			HttpClient: httpclient,
		})
		Expect(err).NotTo(HaveOccurred())

		logger := lager.NewLogger("fetcher-test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

		cfg = &fetchers.FetcherConfig{
			CFClient:           cfClient,
			Logger:             logger,
			PaginationWaitTime: 0,
		}
	})

	Describe("FetchCFAuditEvents", func() {
		It("appears to work", func() {
			q := "timestamp>2019-10-04T12:40:43Z"
			numberOfPages := 10

			resultsChan := make(chan fetchers.CFAuditEventResult, 10)
			eventPages := randomEventPages(numberOfPages, 5)
			pullEventsSince := time.Date(2019, 10, 4, 12, 40, 43, 0, time.UTC)

			for mockIndex := 1; mockIndex <= numberOfPages; mockIndex++ {

				query := url.Values{
					"q":                []string{q},
					"results-per-page": []string{"100"},
				}

				if mockIndex > 1 {
					query["page"] = []string{fmt.Sprintf("%d", mockIndex)}
				}

				nextURL := ""

				if mockIndex < numberOfPages {
					nextURLQuery := url.Values{
						"q":                []string{q},
						"results-per-page": []string{"100"},
					}

					nextURLQuery["page"] = []string{fmt.Sprintf("%d", mockIndex+1)}

					nextURL = fmt.Sprintf(
						"/v2/events?%s", nextURLQuery.Encode(),
					)
				}

				httpmock.RegisterResponderWithQuery(
					"GET",
					fmt.Sprintf("%s/v2/events", cfAPIURL),
					query,
					httpmock.NewJsonResponderOrPanic(200, wrapEventsForResponse(
						numberOfPages, nextURL, eventPages[mockIndex-1],
					)),
				)
			}

			go func() {
				defer GinkgoRecover()
				fetchers.FetchCFAuditEvents(cfg, pullEventsSince, resultsChan)
			}()

			Eventually(httpmock.GetTotalCallCount).Should(Equal(numberOfPages + 2))

			for page := 1; page <= numberOfPages; page++ {
				Expect(resultsChan).To(Receive(
					Equal(fetchers.CFAuditEventResult{Events: eventPages[page-1]}),
				))
			}

			Eventually(resultsChan).Should(BeClosed())
		})
	})

	// Describe("fetchEvents", func() {
	// 	It("outputs events before fetching the next page", func() {
	// 		numberOfPages := 133
	// 		eventPages := randomEventPages(numberOfPages, 5)
	// 		resultsChan := make(chan CFAuditEventResult, 10)

	// 		fakeCFClient.DoRequestStub = func(req *cfclient.Request) (*http.Response, error) {
	// 			page, err := strconv.Atoi(req.Url[1:])
	// 			Expect(err).ToNot(HaveOccurred())
	// 			Expect(page).ToNot(BeNumerically(">=", numberOfPages))

	// 			Expect(resultsChan).To(Receive(Equal(CFAuditEventResult{Events: eventPages[page-1]})))
	// 			Expect(resultsChan).ToNot(Receive())

	// 			nextPageUrl := fmt.Sprintf("/%d", page+1)
	// 			if page+1 >= numberOfPages {
	// 				nextPageUrl = ""
	// 			}
	// 			return eventsStubHTTPResponse(numberOfPages, nextPageUrl, eventPages[page]), nil
	// 		}

	// 		resultsChan <- CFAuditEventResult{Events: eventPages[0]}
	// 		go func() {
	// 			defer GinkgoRecover()
	// 			fetchEvents(cfg, "/1", resultsChan)
	// 		}()

	// 		Eventually(fakeCFClient.DoRequestCallCount).Should(Equal(numberOfPages - 1))
	// 		Eventually(resultsChan).Should(BeClosed())
	// 	})

	// 	It("stops fetching pages when an error is encountered", func() {
	// 		resultsChan := make(chan CFAuditEventResult, 10)
	// 		pageOneEvents := randomEvents(10)

	// 		fakeCFClient.DoRequestStub = func(req *cfclient.Request) (*http.Response, error) {
	// 			if req.Url == "/this-page-will-error" {
	// 				return nil, fmt.Errorf("this page errors")
	// 			}
	// 			Expect(req.Url).To(Equal("/this-page-will-succeed"))
	// 			return eventsStubHTTPResponse(2, "/this-page-will-error", pageOneEvents), nil
	// 		}

	// 		go func() {
	// 			defer GinkgoRecover()
	// 			fetchEvents(cfg, "/this-page-will-succeed", resultsChan)
	// 		}()

	// 		Eventually(fakeCFClient.DoRequestCallCount).Should(Equal(2))
	// 		Expect(resultsChan).To(Receive(Equal(CFAuditEventResult{Events: pageOneEvents})))
	// 		Expect(resultsChan).To(Receive(Equal(CFAuditEventResult{
	// 			Err: fmt.Errorf("error requesting events: this page errors"),
	// 		})))
	// 		Eventually(resultsChan).Should(BeClosed())
	// 	})

	// 	It("sleeps between fetching pages", func() {
	// 		cfg.PaginationWaitTime = 300 * time.Millisecond
	// 		numberOfPages := 5
	// 		resultsChan := make(chan CFAuditEventResult, 10)

	// 		fakeCFClient.DoRequestStub = func(req *cfclient.Request) (*http.Response, error) {
	// 			page, err := strconv.Atoi(req.Url[1:])
	// 			Expect(err).ToNot(HaveOccurred())
	// 			Expect(page).ToNot(BeNumerically(">=", numberOfPages))

	// 			nextPageUrl := fmt.Sprintf("/%d", page+1)
	// 			if page+1 >= numberOfPages {
	// 				nextPageUrl = ""
	// 			}
	// 			return eventsStubHTTPResponse(numberOfPages, nextPageUrl, randomEvents(5)), nil
	// 		}

	// 		go func() {
	// 			defer GinkgoRecover()
	// 			fetchEvents(cfg, "/0", resultsChan)
	// 		}()

	// 		<-resultsChan
	// 		last := time.Now()

	// 		for page := 0; page < numberOfPages; page += 1 {
	// 			result := <-resultsChan
	// 			now := time.Now()
	// 			Expect(result.Err).ToNot(HaveOccurred())
	// 			Expect(now).ToNot(BeTemporally("~", last, cfg.PaginationWaitTime))
	// 			last = now
	// 		}
	// 	})
	// })

	// Describe("getPage", func() {
	// 	It("returns events with the GUID and CreatedAt fields set", func() {
	// 		fakeCFClient.DoRequestReturns(eventsStubHTTPResponse(1, "", []cfclient.Event{cfclient.Event{
	// 			GUID:      "a3e03dd4-3316-4d2e-a4ab-fb941f65e0bd",
	// 			CreatedAt: "2019-09-02T20:02:08Z",
	// 			Type:      "audit.app.create",
	// 		}}), nil)

	// 		_, events, err := getPage(fakeCFClient, "/v2/events")
	// 		Expect(err).ToNot(HaveOccurred())
	// 		Expect(events).To(HaveLen(1))
	// 		Expect(events[0].GUID).To(Equal("a3e03dd4-3316-4d2e-a4ab-fb941f65e0bd"))
	// 		Expect(events[0].CreatedAt).To(Equal("2019-09-02T20:02:08Z"))
	// 		Expect(events[0].Type).To(Equal("audit.app.create"))
	// 	})

	// 	It("returns an error if the request fails", func() {
	// 		fakeCFClient.DoRequestReturns(nil, fmt.Errorf("test error"))

	// 		_, _, err := getPage(fakeCFClient, "/v2/events")
	// 		Expect(err).To(HaveOccurred())
	// 		Expect(err.Error()).To(HaveSuffix("test error"))
	// 	})

	// 	It("returns an error if the response has a non-200 status code", func() {
	// 		fakeCFClient.DoRequestReturns(stubHTTPResponse(401, []byte("Unauthorized")), nil)

	// 		_, _, err := getPage(fakeCFClient, "/v2/events")
	// 		Expect(err).To(HaveOccurred())
	// 		Expect(err.Error()).To(HaveSuffix("401"))
	// 	})
})
