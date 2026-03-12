//go:build functional

package functionaltests

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/andrewshostak/result-service/testutils"
)

const secretKey = "OQJdhUG6twIePy5HWwOu1lqU"

func (s *FunctionalTestSuite) TestSearchAliases_Success() {
	aliases := []string{"Arsenal", "Barcelona", "Juventus"}

	for i := range aliases {
		_, _ = testutils.SetupTeamWithRelations(s.T(), s.db, aliases[i], i+1)
	}

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

	s.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response struct {
		Aliases []string `json:"aliases"`
	}
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal(2, len(response.Aliases))
	s.Equal(response.Aliases[0], aliases[0])
	s.Equal(response.Aliases[1], aliases[1])
}

func (s *FunctionalTestSuite) TestSearchAliases_NothingFound() {
	aliases := []string{"Arsenal", "Barcelona", "Juventus"}

	for i := range aliases {
		_, _ = testutils.SetupTeamWithRelations(s.T(), s.db, aliases[i], i+1)
	}

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

	s.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var response struct {
		Aliases []string `json:"aliases"`
	}
	err = json.Unmarshal(body, &response)
	s.Require().NoError(err)
	s.Equal(0, len(response.Aliases))
}
