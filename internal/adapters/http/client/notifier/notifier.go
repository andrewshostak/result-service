package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/andrewshostak/result-service/internal/app/models"
)

const notificationAuthHeader = "Authorization"

type NotifierClient struct {
	httpClient *http.Client
	logger     Logger
}

func NewNotifierClient(httpClient *http.Client, logger Logger) *NotifierClient {
	return &NotifierClient{httpClient: httpClient, logger: logger}
}

func (c *NotifierClient) Notify(ctx context.Context, notification models.SubscriberNotification) error {
	body := NotificationBody{
		Home: notification.Home,
		Away: notification.Away,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal notify subscriber request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, notification.Url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request to notify subscriber: %w", err)
	}

	req.Header.Set(notificationAuthHeader, notification.Key)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to notify subscribers: %w", err)
	}

	defer func() {
		err := res.Body.Close()
		if err != nil {
			c.logger.Error().Err(err).Msg("couldn't close response body")
		}
	}()

	if res.StatusCode >= http.StatusOK && res.StatusCode <= http.StatusNoContent {
		return nil
	}

	return errors.New(fmt.Sprintf("failed to notify subscribers, status code %d", res.StatusCode))
}
