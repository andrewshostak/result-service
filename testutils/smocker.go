package testutils

import (
	"bytes"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type mockSettings struct {
	baseURL      string
	method       string
	path         string
	queryParams  url.Values
	statusCode   int
	responseBody any
}

type MockOption func(*mockSettings)

func WithMethod(method string) MockOption {
	return func(s *mockSettings) {
		s.method = method
	}
}

func WithStatusCode(statusCode int) MockOption {
	return func(s *mockSettings) {
		s.statusCode = statusCode
	}
}

func WithResponseBody(responseBody any) MockOption {
	return func(s *mockSettings) {
		s.responseBody = responseBody
	}
}

func WithQueryParams(queryParams url.Values) MockOption {
	return func(s *mockSettings) {
		s.queryParams = queryParams
	}
}

func MockHTTPRequest(t *testing.T, baseUrl, path string, opts ...MockOption) {
	t.Helper()

	settings := &mockSettings{
		method:     http.MethodGet,
		statusCode: http.StatusOK,
		path:       path,
		baseURL:    baseUrl,
	}

	for _, opt := range opts {
		opt(settings)
	}

	payload := smockerExpectation{
		Request: mockRequest{
			Path:        settings.path,
			Method:      settings.method,
			QueryParams: map[string][]string{},
		},
		Response: mockResponse{
			Status: settings.statusCode,
		},
	}

	for key, val := range settings.queryParams {
		payload.Request.QueryParams[key] = val
	}

	if v, ok := settings.responseBody.(string); ok {
		payload.Response.Body = v
	}

	var buf bytes.Buffer
	err := yaml.NewEncoder(&buf).Encode([]smockerExpectation{payload})
	require.NoError(t, err)

	response, err := http.Post(baseUrl+"/mocks", "application/x-yaml", &buf)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
}

type smockerExpectation struct {
	Request  mockRequest  `yaml:"request"`
	Response mockResponse `yaml:"response"`
}

type mockRequest struct {
	Method      string              `yaml:"method"`
	Path        string              `yaml:"path"`
	QueryParams map[string][]string `yaml:"query_params,omitempty"`
	Body        interface{}         `yaml:"body,omitempty"`
}

type mockResponse struct {
	Status int    `yaml:"status"`
	Body   string `yaml:"body,omitempty"`
}
