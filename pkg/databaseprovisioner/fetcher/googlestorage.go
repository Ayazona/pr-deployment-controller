package fetcher

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
)

type googleStorageFetcher struct {
	logger *logrus.Entry
	opts   *FetchOpts

	stopChan chan struct{}
}

// NewGoogleStorageFetcher creates a new fetcher for objects on Google Cloud Storage
func NewGoogleStorageFetcher(logger *logrus.Entry, opts *FetchOpts) (Fetcher, error) {
	return &googleStorageFetcher{
		logger:   logger,
		opts:     opts,
		stopChan: make(chan struct{}, 1),
	}, nil
}

func (g *googleStorageFetcher) Start() error {
	// Create context with cancel hook, used to stop long-running command execution
	ctx, cancel := context.WithCancel(context.TODO())

	startTime := time.Now()
	defer func() {
		g.opts.DumpDownloadSeconds.WithLabelValues(g.opts.DatabaseTemplate).Observe(time.Since(startTime).Seconds())
	}()

	// Cancel context when the the stopChan closes
	go func() {
		<-g.stopChan
		cancel()
	}()

	parts := strings.SplitN(g.opts.Source[5:], "/", 2)
	if len(parts) != 2 {
		return errors.New("could not lookup bucket and object")
	}

	jsonCredentials, err := base64.StdEncoding.DecodeString(g.opts.Credentials)
	if err != nil {
		return err
	}

	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(jsonCredentials))
	if err != nil {
		return err
	}

	f, err := os.Create(g.opts.Destibantion)
	if err != nil {
		return err
	}
	defer f.Close() // nolint: errcheck

	wc, err := client.Bucket(parts[0]).Object(parts[1]).NewReader(ctx)
	if err != nil {
		return err
	}

	if _, err = io.Copy(f, wc); err != nil {
		return err
	}

	return wc.Close()
}

func (g *googleStorageFetcher) Stop() {
	close(g.stopChan)
}
