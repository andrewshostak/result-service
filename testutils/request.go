package testutils

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func MockHTTPRequest(t *testing.T, baseUrl string, path string, method string, returnedStatus int, returnedBody string, queryParams map[string]interface{}) {
	t.Helper()

	mockConfig := []map[string]interface{}{
		{
			"request": map[string]interface{}{
				"method":       method,
				"path":         path,
				"query_params": queryParams,
			},
			"response": map[string]interface{}{
				"status": returnedStatus,
				"body":   returnedBody,
			},
		},
	}

	body, _ := json.Marshal(mockConfig)
	response, err := http.Post(baseUrl+"/mocks", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
}
