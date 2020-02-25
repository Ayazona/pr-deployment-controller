package docker

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
)

type errorDetail struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

type errorLine struct {
	ErrorDetail errorDetail `json:"errorDetail"`
}

func (e *errorLine) Error() error {
	if e.ErrorDetail.Message != "" {
		return errors.New(e.ErrorDetail.Message)
	}
	return nil
}

// checkResponse parses the docker daemon response and returns an error if the response contains an error
func checkResponse(resp io.ReadCloser) error {
	if resp != nil {
		defer resp.Close() // nolint: errcheck
		scanner := bufio.NewScanner(resp)
		for scanner.Scan() {
			var p errorLine
			if err := json.Unmarshal(scanner.Bytes(), &p); err != nil {
				return err
			}
			if err := p.Error(); err != nil {
				return err
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	}

	return nil
}
