package collectors_test

import (
	"context"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cfclient "github.com/cloudfoundry-community/go-cfclient"

	"github.com/alphagov/paas-auditor/pkg/collectors"
	dbfakes "github.com/alphagov/paas-auditor/pkg/db/fakes"
	"github.com/alphagov/paas-auditor/pkg/fetchers"
	h "github.com/alphagov/paas-auditor/pkg/testhelpers"
)

var _ = Describe("CfAuditEventCollector Run", func() {
	var (
		coll    *collectors.CfAuditEventCollector
		logger  lager.Logger
		eventDB *dbfakes.FakeEventDB

		cfAuditEventCollectorEventsCollectedTotal float64
	)

	BeforeEach(func() {
		logger = lager.NewLogger("collector-test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

		By("checking the value of the metrics to test against them later")
		cfAuditEventCollectorEventsCollectedTotal = h.CurrentMetricValue(
			collectors.CFAuditEventCollectorEventsCollectedTotal,
		)
	})

	It("appears to work", func() {
		eventDB = &dbfakes.FakeEventDB{}

		eventsToReceive := []fetchers.CFAuditEventResult{
			fetchers.CFAuditEventResult{Events: []cfclient.Event{cfclient.Event{}}},
			fetchers.CFAuditEventResult{Events: []cfclient.Event{cfclient.Event{}}},
			fetchers.CFAuditEventResult{Events: []cfclient.Event{cfclient.Event{}}},
		}

		fetcher := func(_ time.Time, c chan fetchers.CFAuditEventResult) {
			for _, eventPage := range eventsToReceive {
				c <- eventPage
			}
			time.Sleep(10 * time.Millisecond)
			close(c)
		}

		coll = collectors.NewCfAuditEventCollector(
			10*time.Millisecond,
			logger,
			fetcher,
			eventDB,
		)

		var (
			collectError error
			collectWG    sync.WaitGroup
		)

		collectContext, cancelCollect := context.WithTimeout(
			context.Background(), 100*time.Millisecond,
		)

		By("running the collector")
		collectWG.Add(1)
		go func() {
			defer GinkgoRecover()
			collectError = coll.Run(collectContext)
			collectWG.Done()
		}()

		By("waiting for events to be collected")
		Eventually(
			eventDB.StoreCfAuditEventsCallCount, "100ms", "1ms",
		).Should(BeNumerically("==", 3))

		Expect(eventDB.GetLatestCfEventTimeCallCount()).Should(BeNumerically(">=", 1))

		By("checking the metrics")
		Expect(collectors.CFAuditEventCollectorEventsCollectedTotal).To(
			h.MetricIncrementedBy(cfAuditEventCollectorEventsCollectedTotal, ">=", 3),
		)

		By("cleaning up")
		cancelCollect()
		collectWG.Wait()
		Expect(collectError).NotTo(HaveOccurred())
	})
})
