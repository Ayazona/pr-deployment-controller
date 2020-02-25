package webhook

import (
	"encoding/json"
	"net/http"
)

// errorHandler writes an error as a response
func errorHandler(rw http.ResponseWriter, err error, statusCode ...int) {
	status := 500
	if len(statusCode) >= 1 {
		status = statusCode[0]
	}

	type response struct {
		Error string `json:"error"`
	}

	resp := &response{
		Error: err.Error(),
	}

	body, _ := json.Marshal(resp) // nolint: gosec, gas, errcheck

	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("X-Content-Type-Options", "nosniff")
	rw.WriteHeader(status) // nolint: gosec, gas, errcheck
	rw.Write(body)         // nolint: gosec, gas, errcheck
}
