package fetcher

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sirupsen/logrus"
)

// Fetcher represents the implementation needed to retrieve a postgres
// dump file prior to database restore.
type Fetcher interface {
	Start() error
	Stop()
}

// FetchOpts contains the required options to fetch a database dump
type FetchOpts struct {
	DatabaseTemplate string
	Source           string
	Destibantion     string
	Credentials      string

	DumpDownloadSeconds *prometheus.SummaryVec
}

// New initializes a new fetcher
func New(logger *logrus.Entry, opts *FetchOpts) (Fetcher, error) {
	var fetcher Fetcher
	var err error

	// Select fetcher
	if strings.HasPrefix(opts.Source, "gs://") {
		fetcher, err = NewGoogleStorageFetcher(logger, opts)
	} else {
		err = ErrUnknownFetcher
	}

	return fetcher, err
}
