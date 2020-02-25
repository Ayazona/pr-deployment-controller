package docker

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createResponse(body string) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader([]byte(body)))
}

func TestCheckResponseNoError(t *testing.T) {
	resp := createResponse("{}\n")
	assert.Nil(t, checkResponse(resp))
}

func TestCheckResponseWithError(t *testing.T) {
	resp := createResponse("{\"errorDetail\": {\"message\": \"message\", \"error\": \"error\"}}\n")
	assert.Error(t, checkResponse(resp))
}
