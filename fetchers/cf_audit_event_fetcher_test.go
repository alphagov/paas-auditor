package fetchers

import (
	"net/url"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func stubHTTPResponse(statusCode int, jsonifyToBody interface{}) *http.Response {

}

var _ = Describe("CFAuditEvents Fetcher", func() {
	var err error
	var cfg *FetcherConfig

	Describe("startPageURL", func() {
		var nowURL string
		var parsedNowURL *url.URL

		BeforeEach(func() {
			exampleTime := time.Date(2019, 10, 4, 12, 40, 43, 0, time.UTC)
			nowURL = startPageURL(exampleTime)
			parsedNowURL, err = url.ParseRequestURI(nowURL)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should fetch from /v2/events", func() {
			Expect(parsedNowURL.EscapedPath()).To(Equal("/v2/events"))
		})

		It("should fetch 100 results per page", func() {
			Expect(parsedNowURL.Query()).To(HaveKey("results-per-page"))
			Expect(parsedNowURL.Query()["results-per-page"]).To(HaveLen(1))
			Expect(parsedNowURL.Query()["results-per-page"][0]).To(Equal("100"))
		})

		It("should fetch events from after the specified time", func() {
			Expect(parsedNowURL.Query()).To(HaveKey("q"))
			Expect(parsedNowURL.Query()["q"]).To(HaveLen(1))
			Expect(parsedNowURL.Query()["q"][0]).To(Equal("timestamp>2019-10-04T12:40:43Z"))
		})
	})

	Describe("getPage", func() {
		var fakeCFClient *FakeCloudFoundryClient

		BeforeEach(func() {
			fakeCFClient = &FakeCloudFoundryClient{}
			fakeCFClient.NewRequestStub = func(method string, path string) *cfclient.Request {
				return &cfclient.Request{
					Method: method,
					Url:    path,
				}
			}
			cfg = &FetcherConfig{
				CFClient:           fakeCFClient,
				Logger:             lager.NewLogger("test"),
				PaginationWaitTime: 0,
			}
		})

		It("returns events with the GUID and CreatedAt fields set", func() {
			Expect("implementation").To(Equal("TODO"))
		})

		It("returns the URL of the next page", func() {
			fakeCFClient.DoRequestReturns(stubHTTPResponse(http.StatusOK, cfclient.EventsResponse{
				Pages:   2,
				NextURL: "/v2/events?page=2",
			}), nil)

			nextUrl, _, err := getPage(cfg, "/v2/events")
			Expect(err).ToNot(HaveOccurred())
			Expect(nextUrl).To(Equal("/v2/events?page=2"))
		})

		It("returns an error if the request fails", func() {
			fakeCFClient.DoRequestReturns(nil, "test error")

			_, _, err := getPage(cfg, "/v2/events")
			Expect(err).To(HaveOccurred())
		})

		It("returns an error if the response has a non-200 status code", func() {
			Expect("implementation").To(Equal("TODO"))
		})

		It("returns an error if the response cannot be parsed as JSON", func() {
			Expect("implementation").To(Equal("TODO"))
		})

		It("returns an empty string if there is no next page", func() {
			Expect("implementation").To(Equal("TODO"))
		})

		It("only makes one GET request", func() {
			fakeCFClient.DoRequestReturns(stubHTTPResponse(http.StatusOK, cfclient.EventsResponse{
				Pages:   2,
				NextURL: "/v2/events?page=2",
			}), nil)

			_, _, err := getPage(cfg, "/v2/events")
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeCFClient.DoRequestCallCount()).To(Equal(1))
		})
	})
})
