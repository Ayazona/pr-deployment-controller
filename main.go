package main

import (
	"os"

	log "github.com/sirupsen/logrus"
	"k8s.io/klog"

	"github.com/kolonialno/test-environment-manager/cmd"
)

// Noop klog (kubernetes client logger)
type devNull struct{}

func (dn devNull) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func init() {
	klog.SetOutput(devNull{})
}

func main() {
	// Configure logging
	log.SetOutput(os.Stdout)

	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
