package testutils

import (
	"time"

	"github.com/andrewshostak/result-service/internal/adapters/http/client/fotmob"
	"github.com/andrewshostak/result-service/internal/adapters/http/client/task"
	"github.com/brianvoe/gofakeit/v6"
)

func FakeClientMatch(options ...Option[fotmob.MatchFotmob]) fotmob.MatchFotmob {
	match := fotmob.MatchFotmob{
		ID:       int(gofakeit.Int8()),
		Home:     FakeClientTeam(),
		Away:     FakeClientTeam(),
		StatusID: 1,
		Status: fotmob.StatusFotmob{
			UTCTime: gofakeit.Date().Format(time.RFC3339),
		},
	}

	applyOptions(&match, options...)

	return match
}

func FakeClientTeam(options ...Option[fotmob.TeamFotmob]) fotmob.TeamFotmob {
	team := fotmob.TeamFotmob{
		ID:       int(gofakeit.Int8()),
		Score:    gofakeit.IntRange(0, 9),
		Name:     gofakeit.Name(),
		LongName: gofakeit.Name(),
	}

	applyOptions(&team, options...)

	return team
}

func FakeClientClientTask(options ...Option[task.Task]) task.Task {
	task := task.Task{
		Name:      gofakeit.Name(),
		ExecuteAt: time.Now().Add(time.Duration(gofakeit.RandomInt([]int{1, 2, 4, 8})) * time.Hour),
	}

	applyOptions(&task, options...)

	return task
}
