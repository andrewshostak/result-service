package testutils

import (
	"time"

	"github.com/andrewshostak/result-service/internal/adapters/http/client/fotmob"
	"github.com/brianvoe/gofakeit/v6"
)

func FakeMatchesResponse(options ...Option[fotmob.MatchesResponse]) fotmob.MatchesResponse {
	response := fotmob.MatchesResponse{
		Leagues: []fotmob.League{FakeClientLeague()},
	}

	applyOptions(&response, options...)

	return response
}

func FakeClientLeague(options ...Option[fotmob.League]) fotmob.League {
	league := fotmob.League{
		Ccode:            "ENG",
		Name:             "Premier League",
		ParentLeagueName: "Premier League",
		Matches:          []fotmob.Match{FakeClientMatch()},
	}

	applyOptions(&league, options...)

	return league
}

func FakeClientMatch(options ...Option[fotmob.Match]) fotmob.Match {
	match := fotmob.Match{
		ID:       int(gofakeit.Int8()),
		Home:     FakeClientTeam(),
		Away:     FakeClientTeam(),
		StatusID: 1,
		Status: fotmob.Status{
			UTCTime: gofakeit.Date().Format(time.RFC3339),
			Reason: fotmob.Reason{
				Short:    "FT",
				ShortKey: "fulltime_short",
				Long:     "Full-Time",
				LongKey:  "finished",
			},
		},
	}

	applyOptions(&match, options...)

	return match
}

func FakeClientTeam(options ...Option[fotmob.Team]) fotmob.Team {
	team := fotmob.Team{
		ID:       int(gofakeit.Int8()),
		Score:    gofakeit.IntRange(0, 9),
		Name:     gofakeit.Name(),
		LongName: gofakeit.Name(),
	}

	applyOptions(&team, options...)

	return team
}
