package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const matchesPath = "/api/data/matches"

const dateFormat = "20060102"

type FotmobClient struct {
	httpClient *http.Client
	logger     Logger
	baseURL    string
}

func NewFotmobClient(httpClient *http.Client, logger Logger, baseURL string) *FotmobClient {
	return &FotmobClient{httpClient: httpClient, logger: logger, baseURL: baseURL}
}

func (c *FotmobClient) GetMatchesByDate(ctx context.Context, date time.Time) (*MatchesResponse, error) {
	url := c.baseURL + matchesPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to get matches by date: %w", err)
	}

	q := req.URL.Query()
	q.Add("date", date.Format(dateFormat))
	req.URL.RawQuery = q.Encode()

	req.Header.Set("User-Agent", "golang-app")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to get matches by date: %w", err)
	}

	defer func() {
		err := res.Body.Close()
		if err != nil {
			c.logger.Error().Err(err).Msg("couldn't close response body")
		}
	}()

	if res.StatusCode == http.StatusOK {
		var body MatchesResponse
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("failed to decode get matches by date response body: %w", err)
		}

		return &body, nil
	}

	return nil, errors.New(fmt.Sprintf("failed to get matches by date, status code %d", res.StatusCode))
}
