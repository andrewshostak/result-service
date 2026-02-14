package fotmob_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/internal/adapters/http/client/fotmob"
	"github.com/andrewshostak/result-service/internal/adapters/http/client/fotmob/mocks"
	"github.com/andrewshostak/result-service/internal/app/models"
	loggerinternal "github.com/andrewshostak/result-service/internal/infra/logger"
	"github.com/andrewshostak/result-service/testutils"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFotmobClient_GetMatches(t *testing.T) {
	ctx := context.Background()

	cfg := config.ExternalAPI{
		FotmobAPIBaseURL: gofakeit.URL(),
		Timezone:         "Europe/London",
	}

	date := gofakeit.Date()
	dateFormat := "20060102"

	reqUrl := cfg.FotmobAPIBaseURL + fmt.Sprintf("/api/data/matches?date=%s&timezone=%s", date.Format(dateFormat), url.QueryEscape(cfg.Timezone))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "golang-app")

	league := testutils.FakeClientLeague(func(l *fotmob.League) {
		l.Matches = []fotmob.Match{
			testutils.FakeClientMatch(func(m *fotmob.Match) {
				m.StatusID = 6
			}),
			testutils.FakeClientMatch(func(m *fotmob.Match) {
				m.StatusID = 1
			}),
		}
	})

	response := testutils.FakeMatchesResponse(func(f *fotmob.MatchesResponse) {
		f.Leagues = []fotmob.League{league}
	})
	responseBody, err := json.Marshal(response)
	require.NoError(t, err)

	matchWithInvalidDate := testutils.FakeClientMatch(func(m *fotmob.Match) {
		m.Status.UTCTime = "!@#$"
	})
	responseWithInvalidMatch := testutils.FakeMatchesResponse(func(f *fotmob.MatchesResponse) {
		f.Leagues = []fotmob.League{testutils.FakeClientLeague(func(l *fotmob.League) {
			l.Matches = []fotmob.Match{matchWithInvalidDate}
		})}
	})
	responseBodyWithInvalidMatch, err := json.Marshal(responseWithInvalidMatch)
	require.NoError(t, err)

	tests := []struct {
		name        string
		httpManager func(t *testing.T) fotmob.HTTPManager
		result      []models.ExternalAPIMatch
		expectedErr error
	}{
		{
			name: "success - it returns matches if response body and status code are correct",
			httpManager: func(t *testing.T) fotmob.HTTPManager {
				t.Helper()
				httpManager := mocks.NewHTTPManager(t)
				httpManager.
					On("Do", mock.MatchedBy(func(actual *http.Request) bool {
						return testutils.CompareRequest(t, req, actual)
					})).
					Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBuffer(responseBody))}, nil).
					Once()
				return httpManager
			},
			result: []models.ExternalAPIMatch{
				expectedExternalAPIMatch(t, response.Leagues[0].Matches[0]),
				expectedExternalAPIMatch(t, response.Leagues[0].Matches[1]),
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
			expectedErr: errors.New("failed to fetch matches by date: failed to send request to get matches by date: some error"),
		},
		{
			name: "it returns an error if response status code is not ok",
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
			expectedErr: errors.New(fmt.Sprintf("failed to fetch matches by date: failed to get matches by date, status code %d", http.StatusServiceUnavailable)),
		},
		{
			name: "it returns an error if mapping to domain model fails",
			httpManager: func(t *testing.T) fotmob.HTTPManager {
				t.Helper()
				httpManager := mocks.NewHTTPManager(t)
				httpManager.
					On("Do", mock.MatchedBy(func(actual *http.Request) bool {
						return testutils.CompareRequest(t, req, actual)
					})).
					Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBuffer(responseBodyWithInvalidMatch))}, nil).
					Once()
				return httpManager
			},
			expectedErr: fmt.Errorf("failed to map fotmob response to matches: unable to parse match starting time !@#$: parsing time \"!@#$\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"!@#$\" as \"2006\""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := loggerinternal.SetupLogger()

			client := fotmob.NewFotmobClient(tt.httpManager(t), logger, cfg)

			result, err := client.GetMatches(ctx, date)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.result, result)
		})
	}
}

func TestFotmobClient_GetTeams(t *testing.T) {
	ctx := context.Background()

	cfg := config.ExternalAPI{
		FotmobAPIBaseURL: gofakeit.URL(),
		Timezone:         "Europe/London",
	}

	date := gofakeit.Date()
	dateFormat := "20060102"

	reqUrl := cfg.FotmobAPIBaseURL + fmt.Sprintf("/api/data/matches?date=%s&timezone=%s", date.Format(dateFormat), url.QueryEscape(cfg.Timezone))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "golang-app")

	league := testutils.FakeClientLeague(func(l *fotmob.League) {
		l.Matches = []fotmob.Match{
			testutils.FakeClientMatch(),
			testutils.FakeClientMatch(),
		}
	})

	response := testutils.FakeMatchesResponse(func(f *fotmob.MatchesResponse) {
		f.Leagues = []fotmob.League{league}
	})
	responseBody, err := json.Marshal(response)
	require.NoError(t, err)

	tests := []struct {
		name        string
		httpManager func(t *testing.T) fotmob.HTTPManager
		result      []models.ExternalAPITeam
		expectedErr error
	}{
		{
			name: "success - it returns teams if response body and status code are correct",
			httpManager: func(t *testing.T) fotmob.HTTPManager {
				t.Helper()
				httpManager := mocks.NewHTTPManager(t)
				httpManager.
					On("Do", mock.MatchedBy(func(actual *http.Request) bool {
						return testutils.CompareRequest(t, req, actual)
					})).
					Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBuffer(responseBody))}, nil).
					Once()
				return httpManager
			},
			result: expectedExternalAPITeams(response),
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
			expectedErr: errors.New("failed to fetch matches by date: failed to send request to get matches by date: some error"),
		},
		{
			name: "it returns an error if response status code is not ok",
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
			expectedErr: errors.New(fmt.Sprintf("failed to fetch matches by date: failed to get matches by date, status code %d", http.StatusServiceUnavailable)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := loggerinternal.SetupLogger()

			client := fotmob.NewFotmobClient(tt.httpManager(t), logger, cfg)

			result, err := client.GetTeams(ctx, date)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.result, result)
		})
	}
}

func expectedExternalAPITeams(response fotmob.MatchesResponse) []models.ExternalAPITeam {
	var teams []models.ExternalAPITeam
	seen := make(map[int]bool)

	for _, league := range response.Leagues {
		for _, m := range league.Matches {
			if !seen[m.Home.ID] {
				seen[m.Home.ID] = true
				teams = append(teams, models.ExternalAPITeam{
					ID:          m.Home.ID,
					Name:        m.Home.Name,
					LeagueNames: []string{league.Name, league.ParentLeagueName},
					CountryCode: league.Ccode,
				})
			}
			if !seen[m.Away.ID] {
				seen[m.Away.ID] = true
				teams = append(teams, models.ExternalAPITeam{
					ID:          m.Away.ID,
					Name:        m.Away.Name,
					LeagueNames: []string{league.Name, league.ParentLeagueName},
					CountryCode: league.Ccode,
				})
			}
		}
	}

	return teams
}

func expectedExternalAPIMatch(t *testing.T, match fotmob.Match) models.ExternalAPIMatch {
	t.Helper()

	expectedTime, err := time.Parse(time.RFC3339, match.Status.UTCTime)
	require.NoError(t, err, "failed to parse match starting time")

	return models.ExternalAPIMatch{
		ID:        match.ID,
		HomeID:    match.Home.ID,
		AwayID:    match.Away.ID,
		HomeScore: match.Home.Score,
		AwayScore: match.Away.Score,
		Time:      expectedTime,
		Status:    fotmob.ToDomainExternalAPIMatchStatus(match.ID, match.StatusID),
	}
}
