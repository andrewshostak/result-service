//go:build functional

package functionaltests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler"
	"github.com/andrewshostak/result-service/internal/adapters/repository"
	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/andrewshostak/result-service/testutils"
)

func (s *FunctionalTestSuite) TestCreateMatch_AlreadyExistsScheduled() {
	aliases := []string{"Arsenal", "Barcelona", "Juventus"}

	teamIDs := testutils.CreateTeams(s.T(), s.db)

	for i, teamID := range teamIDs {
		testutils.CreateAlias(s.T(), s.db, aliases[i], teamID)
		testutils.CreateExternalTeams(s.T(), s.db, teamID, i+1)
	}

	startsAt, err := time.Parse(time.RFC3339, "2026-01-04T20:00:00Z")
	match := repository.Match{
		StartsAt:     startsAt,
		HomeTeamID:   uint(teamIDs[0]),
		AwayTeamID:   uint(teamIDs[1]),
		ResultStatus: string(models.Scheduled),
	}
	created := testutils.CreateMatch(s.T(), s.db, match)

	s.Require().NoError(err)

	requestPayload := handler.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: "Arsenal",
		AliasAway: "Barcelona",
	}

	requestBody, err := json.Marshal(&requestPayload)
	s.Require().NoError(err)

	url := s.apiBaseURL + "/v1/matches"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response struct {
		MatchID uint `json:"match_id"`
	}
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal(created.ID, response.MatchID)
}
