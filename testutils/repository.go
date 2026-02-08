package testutils

import (
	"github.com/andrewshostak/result-service/internal/adapters/repository"
	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/brianvoe/gofakeit/v6"
)

func FakeRepositoryMatch(options ...Option[repository.Match]) repository.Match {
	statuses := []models.ResultStatus{
		models.NotScheduled,
		models.Scheduled,
		models.SchedulingError,
		models.Received,
		models.APIError,
		models.Cancelled,
	}

	m := repository.Match{
		ID:           uint(gofakeit.Uint8()),
		HomeTeamID:   uint(gofakeit.Uint8()),
		AwayTeamID:   uint(gofakeit.Uint8()),
		StartsAt:     gofakeit.Date(),
		ResultStatus: string(statuses[gofakeit.IntRange(0, len(statuses)-1)]),
	}

	applyOptions(&m, options...)

	return m
}

func FakeRepositoryAlias(options ...Option[repository.Alias]) repository.Alias {
	alias := repository.Alias{
		ID:           uint(gofakeit.Uint8()),
		TeamID:       uint(gofakeit.Uint8()),
		Alias:        gofakeit.Name(),
		ExternalTeam: &repository.ExternalTeam{},
	}

	applyOptions(&alias, options...)

	return alias
}

func FakeExternalMatchRepository(options ...Option[repository.ExternalMatch]) repository.ExternalMatch {
	externalMatch := repository.ExternalMatch{
		ID:        uint(gofakeit.Uint8()),
		MatchID:   uint(gofakeit.Uint8()),
		HomeScore: gofakeit.IntRange(0, 9),
		AwayScore: gofakeit.IntRange(0, 9),
		Status:    gofakeit.RandomString([]string{"not_started", "cancelled", "in_progress", "finished", "unknown"}),
	}

	applyOptions(&externalMatch, options...)

	return externalMatch
}

func FakeRepositorySubscription(options ...Option[repository.Subscription]) repository.Subscription {
	statuses := []models.SubscriptionStatus{
		models.PendingSub,
		models.SchedulingErrorSub,
		models.SuccessfulSub,
		models.SubscriberErrorSub,
	}

	notifiedAt := gofakeit.Date()

	sub := repository.Subscription{
		ID:         uint(gofakeit.Uint8()),
		Url:        gofakeit.URL(),
		MatchID:    uint(gofakeit.Uint8()),
		Key:        gofakeit.Password(true, true, true, false, false, 10),
		Status:     string(statuses[gofakeit.IntRange(0, len(statuses)-1)]),
		NotifiedAt: &notifiedAt,
	}

	applyOptions(&sub, options...)

	return sub
}
