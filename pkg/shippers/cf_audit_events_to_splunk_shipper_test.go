package shippers_test

import (
	"context"
	"net/http"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cfclient "github.com/cloudfoundry-community/go-cfclient"
	"github.com/jarcoal/httpmock"
	"github.com/prometheus/client_golang/prometheus"

	dbfakes "github.com/alphagov/paas-auditor/pkg/db/fakes"
	"github.com/alphagov/paas-auditor/pkg/shippers"
	h "github.com/alphagov/paas-auditor/pkg/testhelpers"
)

const (
	splunkURL = "http://splunk.api/hec-endpoint"
)

var _ = Describe("CFAuditEventsToSplunkShipper Run", func() {
	BeforeSuite(func() {
		httpmock.Activate()
	})

	BeforeEach(func() {
		httpmock.Reset()
	})

	AfterSuite(func() {
		httpmock.DeactivateAndReset()
	})

	var (
		shipper *shippers.CFAuditEventsToSplunkShipper
		logger  lager.Logger
		eventDB *dbfakes.FakeEventDB

		cfAuditEventsToSplunkShipperErrorsTotal        float64
		cfAuditEventsToSplunkShipperEventsShippedTotal float64
	)

	BeforeEach(func() {
		logger = lager.NewLogger("shipper-test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

		By("checking the value of the metrics to test against them later")
		cfAuditEventsToSplunkShipperEventsShippedTotal = h.CurrentMetricValue(
			shippers.CFAuditEventsToSplunkShipperErrorsTotal,
		)
		cfAuditEventsToSplunkShipperEventsShippedTotal = h.CurrentMetricValue(
			shippers.CFAuditEventsToSplunkShipperEventsShippedTotal,
		)

		eventDB = &dbfakes.FakeEventDB{}
		eventDB.GetUnshippedCFAuditEventsForShipperReturns(
			[]cfclient.Event{
				cfclient.Event{GUID: "abcd", CreatedAt: "2006-01-02T15:04:05Z"},
				cfclient.Event{GUID: "efgh", CreatedAt: "2006-01-02T15:04:05Z"},
				cfclient.Event{GUID: "ijkl", CreatedAt: "2006-01-02T15:04:05Z"},
			},
			nil,
		)

		shipper = shippers.NewCFAuditEventsToSplunkShipper(
			10*time.Millisecond,
			logger,
			eventDB,
			"dev", "splunk-key", splunkURL,
		)
	})

	It("appears to work", func() {
		httpmock.RegisterResponder(
			"POST", splunkURL,
			httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{
				"message": "success",
			}),
		)

		var (
			shipError error
			shipWG    sync.WaitGroup
		)

		shipContext, cancelShip := context.WithTimeout(
			context.Background(), 100*time.Millisecond,
		)

		By("running the shipper")
		shipWG.Add(1)
		go func() {
			defer GinkgoRecover()
			shipError = shipper.Run(shipContext)
			shipWG.Done()
		}()

		By("waiting for events to be queried")
		Eventually(
			eventDB.GetUnshippedCFAuditEventsForShipperCallCount, "100ms", "1ms",
		).Should(BeNumerically("==", 1))

		By("waiting for events to be shipped")
		Eventually(
			httpmock.GetTotalCallCount, "1000ms", "1ms",
		).Should(BeNumerically("==", 3))

		Expect(shipError).NotTo(HaveOccurred())

		By("checking the metrics")
		Expect(shippers.CFAuditEventsToSplunkShipperEventsShippedTotal).To(
			h.MetricIncrementedBy(cfAuditEventsToSplunkShipperEventsShippedTotal, ">=", 3),
		)

		By("checking that there were no errors")
		Expect(shippers.CFAuditEventsToSplunkShipperErrorsTotal).To(
			h.MetricIncrementedBy(cfAuditEventsToSplunkShipperErrorsTotal, "==", 0),
		)

		By("cleaning up")
		cancelShip()
		shipWG.Wait()
		Expect(shipError).NotTo(HaveOccurred())
	})

	It("appears is resilient to errors", func() {
		splunkPOSTs := 0

		httpmock.RegisterResponder(
			"POST", splunkURL,
			func(req *http.Request) (*http.Response, error) {
				if splunkPOSTs >= 1 && splunkPOSTs <= 5 {
					splunkPOSTs++
					return httpmock.NewJsonResponse(500, map[string]interface{}{
						"message": "failure",
					})
				}

				splunkPOSTs++
				return httpmock.NewJsonResponse(200, map[string]interface{}{
					"message": "success",
				})
			},
		)

		var (
			shipError error
			shipWG    sync.WaitGroup
		)

		shipContext, cancelShip := context.WithTimeout(
			context.Background(), 10*time.Second,
		)

		By("running the shipper")
		shipWG.Add(1)
		go func() {
			defer GinkgoRecover()
			shipError = shipper.Run(shipContext)
			shipWG.Done()
		}()

		By("waiting for events to be queried")
		Eventually(
			eventDB.GetUnshippedCFAuditEventsForShipperCallCount, "100ms", "1ms",
		).Should(BeNumerically("==", 1))

		By("waiting for events to be shipped")
		Eventually(
			httpmock.GetTotalCallCount, "1000ms", "1ms",
		).Should(BeNumerically(">=", 1))

		By("checking the metrics")
		Eventually(
			func() prometheus.Collector {
				return shippers.CFAuditEventsToSplunkShipperErrorsTotal
			}, "10s", "1ms",
		).Should(
			h.MetricIncrementedBy(cfAuditEventsToSplunkShipperErrorsTotal, ">=", 1),
		)

		By("waiting for events to be queried again")
		Eventually(
			eventDB.GetUnshippedCFAuditEventsForShipperCallCount, "1s", "1ms",
		).Should(BeNumerically("==", 2))

		By("waiting for events to be shipped")
		Eventually(
			httpmock.GetTotalCallCount, "5s", "1ms",
		).Should(BeNumerically(">=", 10))

		By("checking the metrics")
		Expect(shippers.CFAuditEventsToSplunkShipperEventsShippedTotal).To(
			h.MetricIncrementedBy(cfAuditEventsToSplunkShipperEventsShippedTotal, ">=", 2),
		)
		Expect(shippers.CFAuditEventsToSplunkShipperErrorsTotal).To(
			h.MetricIncrementedBy(cfAuditEventsToSplunkShipperErrorsTotal, "==", 1),
		)

		By("cleaning up")
		cancelShip()
		shipWG.Wait()
		Expect(shipError).NotTo(HaveOccurred())
	})
})
