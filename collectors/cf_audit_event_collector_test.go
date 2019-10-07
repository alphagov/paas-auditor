package collectors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/satori/go.uuid"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"code.cloudfoundry.org/lager"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CFAuditEvents Collector", func() {
	var err error
	var fakeEventDB *FakeEventDB
	var cFAuditEventFetcher CFAuditEventFetcher
	var cfAuditEventCollector *CfAuditEventCollector

	BeforeEach(func() {
		fakeEventDB := &FakeEventDB{}
		cFAuditEventFetcher := fetchers.FetchCFAuditEvents
		cfAuditEventCollector :=
	})

	Describe("pullEventsSince", func() {
		It("returns if there is a database error", func() {
			fakeEventDB.GetLatestCfEventTimeReturns(nil, fmt.Errorf("test-error"))
			t, err := cfAuditEventCollector.pullEventsSince(0)
			Expect(t).To(BeNil())
			Expect(err).To(MatchError(fmt.Errorf("test-error")))
		})

		It("returns a zero time if there were no events in the database", func() {
			fakeEventDB.GetLatestCfEventTimeReturns(nil, nil)
			t, err := cfAuditEventCollector.pullEventsSince(0)
			Expect(err).ToNot(HaveOccurred())
			Expect(t).To(Equal(time.Time{}))
		})

		It("adjusts the database's last seen time to overlap", func() {
			fakeEventDB.GetLatestCfEventTimeReturns(27*time.Minute, nil)
			t, err := cfAuditEventCollector.pullEventsSince(11 * time.Second)
			Expect(err).ToNot(HaveOccurred())
			expectedTime := time.Time{}.Add(27 * time.Minute).Add(-11 * time.Second)
			Expect(t).To(Equal(expectedTime))
		})
	})
})
