package status

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
)

func render(w io.Writer, r *http.Request, tpl *template.Template, name string, data interface{}) {
	buf := new(bytes.Buffer)
	if err := tpl.ExecuteTemplate(buf, name, data); err != nil {
		return
	}

	w.Write(buf.Bytes()) // nolint: gas, errcheck
}

func errorHandler(w http.ResponseWriter, err error, statusCode ...int) {
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

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status) // nolint: gosec, gas, errcheck
	w.Write(body)         // nolint: gosec, gas, errcheck
}
