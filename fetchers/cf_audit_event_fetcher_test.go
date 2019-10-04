package fetchers

import (
	"net/url"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CFAuditEvents Fetcher", func() {
	Describe("startPageURL", func() {
		var err error
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
		var err error
		var client *cfclient.Client

		BeforeEach(func() {
			client, err = cfclient.NewClient(&cfclient.Config{
				ApiAddress: "https://cc.internal",
				Username:   "admin",
				Password:   "password",
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("makes one GET request", func() {

		})

		It("returns an error if the request fails", func() {
			Expect(true).To(Equal(false))
		})

		It("returns an error if the response has a non-200 status code", func() {
			Expect(true).To(Equal(false))
		})

		It("returns an error if the response cannot be parsed as JSON", func() {
			Expect(true).To(Equal(false))
		})

		It("returns events with the GUID and CreatedAt fields set", func() {
			Expect(true).To(Equal(false))
		})

		It("returns the URL of the next page", func() {
			Expect(true).To(Equal(false))
		})

		It("returns an empty string if there is no next page", func() {
			Expect(true).To(Equal(false))
		})
	})
})
