package acceptance_test

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry-incubator/cf-test-helpers/generator"
	"github.com/cloudfoundry-incubator/cf-test-helpers/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("paas-auditor acceptance", func() {
	It("is a started app", func() {
		q := url.Values{}
		q.Set("q", "name:paas-auditor")
		apps, err := cfclient.ListAppsByQuery()
		Expect(err).ToNot(HaveOccurred())
		Expect(apps).To(HaveLength(1))
		Expect(apps[0].State).To(Equal("started"))
	})

	It("stores new audit events within 7.5 minutes", func() {
		q := url.Values{}
		q.Set("q", fmt.Sprintf("timestamp>%s", time.Now()-5*time.Minute))
		events, err := cfclient.ListEventsByQuery(q)
		Expect(err).ToNot(HaveOccurred())

		// connect to paas-auditor's DB [how?]
		// ummmm. conduit? conduit would work. but this really calls for an API to call.
		// could the acceptance tests be deployed as a task that gets bound to the DB?
		// It probably is best to add an API to paas-auditor which requires basic auth
		// credentials.

		// check that all those events make it in by polling for 7.5 minutes

	})
})
