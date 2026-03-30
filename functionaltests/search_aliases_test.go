//go:build functional

package functionaltests

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler"
	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/andrewshostak/result-service/testutils"
)

const secretKey = "OQJdhUG6twIePy5HWwOu1lqU"

func (s *FunctionalTestSuite) TestSearchAliases_Success() {
	teamSeeds := testutils.SetupTeamsWithRelations(s.T(), s.db)

	url := s.apiBaseURL + "/v1/aliases"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	q := req.URL.Query()
	q.Add("search", "ar")
	req.URL.RawQuery = q.Encode()

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response struct {
		Aliases []string `json:"aliases"`
	}
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal(2, len(response.Aliases))
	s.Equal(teamSeeds[0].Alias, response.Aliases[0])
	s.Equal(teamSeeds[1].Alias, response.Aliases[1])
}

func (s *FunctionalTestSuite) TestSearchAliases_InvalidRequest() {
	_ = testutils.SetupTeamsWithRelations(s.T(), s.db)

	url := s.apiBaseURL + "/v1/aliases"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Require().Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response handler.ErrorResponse
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Contains(response.Error, "required")
	s.Equal(string(models.CodeInvalidRequest), response.Code)
}

func (s *FunctionalTestSuite) TestSearchAliases_NothingFound() {
	_ = testutils.SetupTeamsWithRelations(s.T(), s.db)

	url := s.apiBaseURL + "/v1/aliases"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", secretKey)

	q := req.URL.Query()
	q.Add("search", "Manchester")
	req.URL.RawQuery = q.Encode()

	resp, err := s.httpClient.Do(req)
	s.Require().NoError(err)

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response struct {
		Aliases []string `json:"aliases"`
	}
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal(0, len(response.Aliases))
}
