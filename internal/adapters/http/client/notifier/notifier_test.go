package notifier_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/andrewshostak/result-service/internal/adapters/http/client/fotmob"
	"github.com/andrewshostak/result-service/internal/adapters/http/client/notifier"
	"github.com/andrewshostak/result-service/internal/adapters/http/client/notifier/mocks"
	"github.com/andrewshostak/result-service/internal/app/models"
	loggerinternal "github.com/andrewshostak/result-service/internal/infra/logger"
	"github.com/andrewshostak/result-service/testutils"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNotifierClient_Notify(t *testing.T) {
	ctx := context.Background()

	subscriberNotification := models.SubscriberNotification{
		Url:  gofakeit.URL(),
		Key:  gofakeit.Password(true, true, true, false, false, 10),
		Home: uint(gofakeit.Uint8()),
		Away: uint(gofakeit.Uint8()),
	}

	body := notifier.NotificationBody{
		Home: subscriberNotification.Home,
		Away: subscriberNotification.Away,
	}

	requestBody, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, subscriberNotification.Url, bytes.NewReader(requestBody))
	require.NoError(t, err)
	req.Header.Set("Authorization", subscriberNotification.Key)
	req.Header.Set("Content-Type", "application/json")

	tests := []struct {
		name        string
		httpManager func(t *testing.T) fotmob.HTTPManager
		expectedErr error
	}{
		{
			name: "success - it returns no error if response code is 2xx",
			httpManager: func(t *testing.T) fotmob.HTTPManager {
				t.Helper()
				httpManager := mocks.NewHTTPManager(t)
				httpManager.
					On("Do", mock.MatchedBy(func(actual *http.Request) bool {
						return testutils.CompareRequest(t, req, actual)
					})).
					Return(&http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil).
					Once()
				return httpManager
			},
		},
		{
			name: "it returns an error when fails to make a request",
			httpManager: func(t *testing.T) fotmob.HTTPManager {
				t.Helper()
				httpManager := mocks.NewHTTPManager(t)
				httpManager.
					On("Do", mock.MatchedBy(func(actual *http.Request) bool {
						return testutils.CompareRequest(t, req, actual)
					})).
					Return(nil, errors.New("some error")).
					Once()
				return httpManager
			},
			expectedErr: fmt.Errorf("failed to send request to notify subscribers: %w", errors.New("some error")),
		},
		{
			name: "it returns an error when response code is not 2xx",
			httpManager: func(t *testing.T) fotmob.HTTPManager {
				t.Helper()
				httpManager := mocks.NewHTTPManager(t)
				httpManager.
					On("Do", mock.MatchedBy(func(actual *http.Request) bool {
						return testutils.CompareRequest(t, req, actual)
					})).
					Return(&http.Response{StatusCode: http.StatusServiceUnavailable, Body: http.NoBody}, nil).
					Once()
				return httpManager
			},
			expectedErr: errors.New(fmt.Sprintf("failed to notify subscribers, status code %d", http.StatusServiceUnavailable)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := loggerinternal.SetupLogger()

			client := notifier.NewNotifierClient(tt.httpManager(t), logger)

			err := client.Notify(ctx, subscriberNotification)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
