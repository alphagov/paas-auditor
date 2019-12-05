package fetchers

import (
	"time"

	"code.cloudfoundry.org/lager"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

type FetcherConfig struct {
	CFClient           cfclient.CloudFoundryClient
	Logger             lager.Logger
	PaginationWaitTime time.Duration
}
