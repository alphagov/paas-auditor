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

func mockEventPageResponse(
	page int, totalPages int, addNextURL bool,
	expectedQ string,
	events []cfclient.Event,
) {
	var nextURL string
	mockURL := fmt.Sprintf("%s/v2/events", cfAPIURL)

	expectedQuery := url.Values{
		"q":                []string{expectedQ},
		"results-per-page": []string{"100"},
	}

	if page > 1 {
		expectedQuery["page"] = []string{fmt.Sprintf("%d", page)}
	}

	if addNextURL {
		nextURLQuery := url.Values{
			"q":                []string{expectedQ},
			"results-per-page": []string{"100"},
		}

		nextURLQuery["page"] = []string{fmt.Sprintf("%d", page+1)}

		nextURL = fmt.Sprintf(
			"/v2/events?%s", nextURLQuery.Encode(),
		)
	}

	resp := httpmock.NewJsonResponderOrPanic(
		200, wrapEventsForResponse(totalPages, nextURL, events),
	)
	httpmock.RegisterResponderWithQuery("GET", mockURL, expectedQuery, resp)
}

func wrapEventsForResponse(
	pages int,
	nextURL string,
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
		NextURL:      nextURL,
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
	for i := 0; i < n; i++ {
		events[i] = randomEvent()
	}
	return events
}

func randomEventPages(numberOfPages, eventsPerPage int) [][]cfclient.Event {
	eventPages := make([][]cfclient.Event, numberOfPages)
	for page := 0; page < numberOfPages; page++ {
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

		httpmock.Reset() // Reset mock after client creation to clear call count

		logger := lager.NewLogger("fetcher-test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

		cfg = &fetchers.FetcherConfig{
			CFClient:           cfClient,
			Logger:             logger,
			PaginationWaitTime: 10 * time.Millisecond,
		}
	})

	Describe("FetchCFAuditEvents", func() {
		const (
			numberOfPages = 10
		)

		var (
			resultsChan chan fetchers.CFAuditEventResult
			eventPages  [][]cfclient.Event
		)

		BeforeEach(func() {
			resultsChan = make(chan fetchers.CFAuditEventResult, numberOfPages)
			eventPages = randomEventPages(numberOfPages, 5)
		})

		It("appears to work", func() {
			expectedQ := "timestamp>2019-10-04T12:40:43Z"
			pullEventsSince := time.Date(2019, 10, 4, 12, 40, 43, 0, time.UTC)

			By("registering mocks")
			for page := 1; page <= numberOfPages; page++ {
				thereAreMorePages := page != numberOfPages

				mockEventPageResponse(
					page, numberOfPages, thereAreMorePages,
					expectedQ,
					eventPages[page-1],
				)
			}

			By("fetching events")
			go func() {
				defer GinkgoRecover()
				fetchers.FetchCFAuditEvents(cfg, pullEventsSince, resultsChan)
			}()

			By("expecting results via the channel")
			for page := 1; page <= numberOfPages; page++ {
				Eventually(resultsChan, "100ms", "1ms").Should(Receive(
					Equal(fetchers.CFAuditEventResult{Events: eventPages[page-1]}),
				))

				Expect(httpmock.GetTotalCallCount()).To(Equal(page))
			}

			By("checking we are finished")
			Eventually(resultsChan).Should(BeClosed())
			Eventually(httpmock.GetTotalCallCount).Should(Equal(numberOfPages))
		})

		It("returns an error and closes the chan when there is an error", func() {
			expectedQ := "timestamp>2019-10-04T12:40:43Z"
			pullEventsSince := time.Date(2019, 10, 4, 12, 40, 43, 0, time.UTC)

			By("registering mocks")
			// Mock the first two pages
			for p := 1; p <= 2; p++ {
				mockEventPageResponse(p, numberOfPages, true, expectedQ, eventPages[p])
			}
			// The next request will fail
			httpmock.RegisterResponder(
				"GET", fmt.Sprintf(`=~^%s.*\z`, cfAPIURL),
				func(req *http.Request) (*http.Response, error) {
					return &http.Response{}, fmt.Errorf("Network error")
				},
			)

			By("fetching events")
			go func() {
				defer GinkgoRecover()
				fetchers.FetchCFAuditEvents(cfg, pullEventsSince, resultsChan)
			}()

			By("expecting results via the channel")
			for p := 1; p <= 2; p++ {
				Eventually(resultsChan, "100ms", "1ms").Should(Receive(
					Equal(fetchers.CFAuditEventResult{Events: eventPages[p]}),
				))
			}
			Eventually(resultsChan, "100ms", "1ms").Should(Receive(WithTransform(
				func(res fetchers.CFAuditEventResult) error { return res.Err },
				MatchError(ContainSubstring("Network error")),
			)))

			By("checking we are finished")
			Eventually(resultsChan).Should(BeClosed())
			Eventually(httpmock.GetTotalCallCount).Should(Equal(3))
		})
	})

	// Describe("fetchEvents", func() {
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

	// 	It("returns an error if the response has a non-200 status code", func() {
	// 		fakeCFClient.DoRequestReturns(stubHTTPResponse(401, []byte("Unauthorized")), nil)

	// 		_, _, err := getPage(fakeCFClient, "/v2/events")
	// 		Expect(err).To(HaveOccurred())
	// 		Expect(err.Error()).To(HaveSuffix("401"))
	// 	})
})
