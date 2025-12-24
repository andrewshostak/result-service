package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/andrewshostak/result-service/errs"
)

const (
	fixturesPath = "/v3/fixtures"
	leaguesPath  = "/v3/leagues"
	teamsPath    = "/v3/teams"
)
const authHeader = "X-RapidAPI-Key"

type FootballAPIClient struct {
	httpClient *http.Client
	logger     Logger
	baseURL    string
	apiKey     string
}

func NewFootballAPIClient(httpClient *http.Client, logger Logger, baseURL string, apiKey string) *FootballAPIClient {
	return &FootballAPIClient{httpClient: httpClient, logger: logger, baseURL: baseURL, apiKey: apiKey}
}

func (c *FootballAPIClient) SearchFixtures(ctx context.Context, search FixtureSearch) (*FixturesResponse, error) {
	url := c.baseURL + fixturesPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to get fixtures: %w", err)
	}

	q := req.URL.Query()
	if search.Timezone != nil {
		q.Add("timezone", *search.Timezone)
	}

	if search.Season != nil {
		q.Add("season", strconv.Itoa(int(*search.Season)))
	}

	if search.TeamID != nil {
		q.Add("team", strconv.Itoa(int(*search.TeamID)))
	}

	if search.Date != nil {
		q.Add("date", *search.Date)
	}

	if search.ID != nil {
		q.Add("id", strconv.Itoa(int(*search.ID)))
	}

	req.URL.RawQuery = q.Encode()

	req.Header.Set(authHeader, c.apiKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to get fixtures: %w", err)
	}

	defer func() {
		err := res.Body.Close()
		if err != nil {
			c.logger.Error().Err(err).Msg("couldn't close response body")
		}
	}()

	if res.StatusCode == http.StatusOK {
		var body FixturesResponse
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("failed to decode get fixtures response body: %w", err)
		}

		return &body, nil
	}

	return nil, fmt.Errorf("%s: %w", fmt.Sprintf("failed to get fixtures, status %d", res.StatusCode), errs.ErrUnexpectedAPIFootballStatusCode)
}

func (c *FootballAPIClient) SearchTeams(ctx context.Context, search TeamsSearch) (*TeamsResponse, error) {
	url := c.baseURL + teamsPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to get teams: %w", err)
	}

	q := req.URL.Query()
	q.Add("season", strconv.Itoa(int(search.Season)))
	q.Add("league", strconv.Itoa(int(search.League)))

	req.URL.RawQuery = q.Encode()

	req.Header.Set(authHeader, c.apiKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to get teams: %w", err)
	}

	defer func() {
		err := res.Body.Close()
		if err != nil {
			c.logger.Error().Err(err).Msg("couldn't close response body")
		}
	}()

	if res.StatusCode == http.StatusOK {
		var body TeamsResponse
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("failed to decode get teams response body: %w", err)
		}

		return &body, nil
	}

	return nil, fmt.Errorf("%s: %w", fmt.Sprintf("failed to get teams, status %d", res.StatusCode), errs.ErrUnexpectedAPIFootballStatusCode)
}
