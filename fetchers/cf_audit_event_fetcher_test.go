package fetchers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	//"code.cloudfoundry.org/lager"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func stubHTTPResponse(statusCode int, jsonifyToBody interface{}) *http.Response {
	json, err := json.MarshalIndent(jsonifyToBody, "", "  ")
	if err != nil {
		panic(err)
	}
	return &http.Response{
		StatusCode: statusCode,
		Body:       ioutil.NopCloser(bytes.NewReader(json)),
	}
}

var _ = Describe("CFAuditEvents Fetcher", func() {
	var err error
	//var cfg *FetcherConfig

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
		})

		// It("returns events with the GUID and CreatedAt fields set", func() {
		// 	Expect("implementation").To(Equal("TODO"))
		// })

		It("returns the URL of the next page", func() {
			fakeCFClient.DoRequestReturns(stubHTTPResponse(200, cfclient.EventsResponse{
				Pages:   2,
				NextURL: "/v2/events?page=2",
			}), nil)

			nextUrl, _, err := getPage(fakeCFClient, "/v2/events")
			Expect(err).ToNot(HaveOccurred())
			Expect(nextUrl).To(Equal("/v2/events?page=2"))
		})

		It("returns an empty string if there is no next page", func() {
			fakeCFClient.DoRequestReturns(stubHTTPResponse(200, cfclient.EventsResponse{NextURL: ""}), nil)

			// TODO: Add a few events and check they did come out as well as the empty nextUrl
			nextUrl, _, err := getPage(fakeCFClient, "/v2/events")
			Expect(err).ToNot(HaveOccurred())
			Expect(nextUrl).To(Equal(""))
		})

		It("returns an error if the request fails", func() {
			fakeCFClient.DoRequestReturns(nil, fmt.Errorf("test error"))

			_, _, err := getPage(fakeCFClient, "/v2/events")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(HaveSuffix("test error"))
		})

		It("returns an error if the response has a non-200 status code", func() {
			fakeCFClient.DoRequestReturns(stubHTTPResponse(401, "Unauthorized"), nil)

			_, _, err := getPage(fakeCFClient, "/v2/events")
			Expect(err).To(HaveOccurred())
		})

		It("only makes one GET request", func() {
			fakeCFClient.DoRequestReturns(stubHTTPResponse(http.StatusOK, cfclient.EventsResponse{
				Pages:   2,
				NextURL: "/v2/events?page=2",
			}), nil)

			_, _, err := getPage(fakeCFClient, "/v2/events")
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeCFClient.DoRequestCallCount()).To(Equal(1))
		})
	})
})
