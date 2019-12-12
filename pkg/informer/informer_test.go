package informer_test

import (
	"context"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	dbfakes "github.com/alphagov/paas-auditor/pkg/db/fakes"
	"github.com/alphagov/paas-auditor/pkg/informer"
	h "github.com/alphagov/paas-auditor/pkg/testhelpers"
)

var _ = Describe("CFAuditEventsToSplunkShipper Run", func() {
	var (
		i       *informer.Informer
		logger  lager.Logger
		eventDB *dbfakes.FakeEventDB

		informerCFAuditEventsTotal          float64
		informerLatestCFAuditEventTimestamp float64
	)

	BeforeEach(func() {
		logger = lager.NewLogger("informer-test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

		By("checking the value of the metrics to test against them later")
		informerCFAuditEventsTotal = h.CurrentMetricValue(
			informer.InformerCFAuditEventsTotal,
		)
		informerLatestCFAuditEventTimestamp = h.CurrentMetricValue(
			informer.InformerLatestCFAuditEventTimestamp,
		)

		currentTime := time.Now()

		eventDB = &dbfakes.FakeEventDB{}
		eventDB.GetCFEventCountReturns(int64(100), nil)
		eventDB.GetLatestCFEventTimeReturns(&currentTime, nil)

		i = informer.NewInformer(
			10*time.Millisecond,
			logger,
			eventDB,
		)
	})

	It("should eventually report some stats about the database", func() {
		var (
			infError error
			infWG    sync.WaitGroup
		)

		infContext, cancelInf := context.WithTimeout(
			context.Background(), 100*time.Millisecond,
		)

		By("running the informer")
		infWG.Add(1)
		go func() {
			defer GinkgoRecover()
			infError = i.Run(infContext)
			infWG.Done()
		}()

		By("waiting for metrics")
		Eventually(
			func() float64 {
				return h.CurrentMetricValue(informer.InformerCFAuditEventsTotal)
			}, "100ms", "1ms",
		).Should(BeNumerically("==", float64(100)))

		By("checking the metrics")
		Expect(informer.InformerCFAuditEventsTotal).To(
			h.MetricIncrementedBy(informerCFAuditEventsTotal, "==", 100),
		)
		Expect(informer.InformerLatestCFAuditEventTimestamp).To(
			h.MetricIncrementedBy(informerLatestCFAuditEventTimestamp, ">", 0),
		)

		By("cleaning up")
		cancelInf()
		infWG.Wait()
		Expect(infError).NotTo(HaveOccurred())
	})
})
