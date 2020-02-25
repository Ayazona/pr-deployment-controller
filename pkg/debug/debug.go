package debug

import (
	"net/http"
	"net/http/pprof"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func healthHandler(rw http.ResponseWriter, r *http.Request) {
	rw.WriteHeader(http.StatusOK) // nolint: gosec, gas
}

// New returns the debug server handlers
func New() (http.Handler, error) {
	m := mux.NewRouter()

	m.Handle("/metrics", promhttp.Handler())
	m.HandleFunc("/health", healthHandler)

	m.HandleFunc("/debug/pprof/", pprof.Index)
	m.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	m.HandleFunc("/debug/pprof/profile", pprof.Profile)
	m.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	m.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return m, nil
}
