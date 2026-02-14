package fotmob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/internal/app/models"
)

const matchesPath = "/api/data/matches"

const dateFormat = "20060102"

type FotmobClient struct {
	httpClient HTTPManager
	logger     Logger
	config     config.ExternalAPI
}

func NewFotmobClient(httpClient HTTPManager, logger Logger, config config.ExternalAPI) *FotmobClient {
	return &FotmobClient{httpClient: httpClient, logger: logger, config: config}
}

func (c *FotmobClient) GetTeams(ctx context.Context, date time.Time) ([]models.ExternalAPITeam, error) {
	response, err := c.fetchMatchesByDate(ctx, date)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch matches by date: %w", err)
	}

	return toDomainExternalAPITeams(*response), nil
}

func (c *FotmobClient) GetMatches(ctx context.Context, date time.Time) ([]models.ExternalAPIMatch, error) {
	response, err := c.fetchMatchesByDate(ctx, date)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch matches by date: %w", err)
	}

	matches, err := toDomainExternalAPIMatches(*response)
	if err != nil {
		return nil, fmt.Errorf("failed to map fotmob response to matches: %w", err)
	}

	return matches, nil
}

func (c *FotmobClient) fetchMatchesByDate(ctx context.Context, date time.Time) (*MatchesResponse, error) {
	url := c.config.FotmobAPIBaseURL + matchesPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to get matches by date: %w", err)
	}

	q := req.URL.Query()
	q.Add("date", date.Format(dateFormat))
	q.Add("timezone", c.config.Timezone)
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
