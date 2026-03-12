package testutils

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func MockHTTPRequest(t *testing.T, baseUrl string, path string, method string, returnedStatus int, returnedBody string) {
	t.Helper()

	mockConfig := []map[string]interface{}{
		{
			"request": map[string]interface{}{
				"method": method,
				"path":   path,
			},
			"response": map[string]interface{}{
				"status": returnedStatus,
				"body":   returnedBody,
			},
		},
	}

	body, _ := json.Marshal(mockConfig)
	_, err := http.Post(baseUrl+"/mocks", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
}
