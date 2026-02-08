package testutils

import (
	"time"

	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/brianvoe/gofakeit/v6"
)

type Option[T any] func(*T)

func applyOptions[T any](item *T, updates ...Option[T]) {
	for _, update := range updates {
		update(item)
	}
}

func FakeAlias(options ...Option[models.Alias]) models.Alias {
	alias := models.Alias{
		TeamID:       uint(gofakeit.Uint8()),
		Alias:        gofakeit.Name(),
		ExternalTeam: &models.ExternalTeam{},
	}

	applyOptions(&alias, options...)

	return alias
}

func FakeMatch(options ...Option[models.Match]) models.Match {
	statuses := []models.ResultStatus{
		models.NotScheduled,
		models.Scheduled,
		models.SchedulingError,
		models.Received,
		models.APIError,
		models.Cancelled,
	}

	externalMatch := FakeExternalMatch(func(m *models.ExternalMatch) {})

	m := models.Match{
		ID:            uint(gofakeit.Uint8()),
		StartsAt:      gofakeit.Date(),
		HomeTeamID:    uint(gofakeit.Uint8()),
		AwayTeamID:    uint(gofakeit.Uint8()),
		ResultStatus:  statuses[gofakeit.IntRange(0, len(statuses)-1)],
		ExternalMatch: &externalMatch,
	}

	applyOptions(&m, options...)

	return m
}

func FakeExternalMatch(options ...Option[models.ExternalMatch]) models.ExternalMatch {
	statuses := []models.ExternalMatchStatus{
		models.StatusMatchNotStarted,
		models.StatusMatchCancelled,
		models.StatusMatchInProgress,
		models.StatusMatchFinished,
		models.StatusMatchUnknown,
	}

	externalMatch := models.ExternalMatch{
		ID:        uint(gofakeit.Uint8()),
		MatchID:   uint(gofakeit.Uint8()),
		HomeScore: gofakeit.IntRange(0, 9),
		AwayScore: gofakeit.IntRange(0, 9),
		Status:    statuses[gofakeit.IntRange(0, len(statuses)-1)],
	}

	applyOptions(&externalMatch, options...)

	return externalMatch
}

func FakeExternalAPIMatch(options ...Option[models.ExternalAPIMatch]) models.ExternalAPIMatch {
	statuses := []models.ExternalMatchStatus{
		models.StatusMatchNotStarted,
		models.StatusMatchCancelled,
		models.StatusMatchInProgress,
		models.StatusMatchFinished,
		models.StatusMatchUnknown,
	}

	externalMatch := models.ExternalAPIMatch{
		ID:     int(gofakeit.Int8()),
		Time:   gofakeit.Date(),
		Home:   FakeExternalAPITeam(),
		Away:   FakeExternalAPITeam(),
		Status: statuses[gofakeit.IntRange(0, len(statuses)-1)],
	}

	applyOptions(&externalMatch, options...)

	return externalMatch
}

func FakeExternalAPITeam(options ...Option[models.ExternalAPITeam]) models.ExternalAPITeam {
	externalTeam := models.ExternalAPITeam{
		ID:    int(gofakeit.Int8()),
		Score: gofakeit.IntRange(0, 9),
		Name:  gofakeit.Name(),
	}

	applyOptions(&externalTeam, options...)

	return externalTeam
}

func FakeTask(options ...Option[models.Task]) models.Task {
	task := models.Task{
		Name:      gofakeit.Name(),
		ExecuteAt: time.Now().Add(time.Duration(gofakeit.RandomInt([]int{1, 2, 4, 8})) * time.Hour),
	}
	applyOptions(&task, options...)

	return task
}

func FakeCheckResultTask(options ...Option[models.CheckResultTask]) models.CheckResultTask {
	task := models.CheckResultTask{
		ID:            uint(gofakeit.Uint8()),
		MatchID:       uint(gofakeit.Uint8()),
		Name:          gofakeit.Name(),
		AttemptNumber: uint(gofakeit.IntRange(1, 9)),
	}

	applyOptions(&task, options...)

	return task
}

func FakeSubscription(options ...Option[models.Subscription]) models.Subscription {
	statuses := []models.SubscriptionStatus{
		models.PendingSub,
		models.SchedulingErrorSub,
		models.SuccessfulSub,
		models.SubscriberErrorSub,
	}

	notifiedAt := gofakeit.Date()

	sub := models.Subscription{
		ID:         uint(gofakeit.Uint8()),
		Url:        gofakeit.URL(),
		MatchID:    uint(gofakeit.Uint8()),
		Key:        gofakeit.Password(true, true, true, false, false, 10),
		Status:     statuses[gofakeit.IntRange(0, len(statuses)-1)],
		NotifiedAt: &notifiedAt,
	}

	applyOptions(&sub, options...)

	return sub
}

func FakeSubscriberNotification(options ...Option[models.SubscriberNotification]) models.SubscriberNotification {
	notification := models.SubscriberNotification{
		Url:  gofakeit.URL(),
		Key:  gofakeit.UUID(),
		Home: uint(gofakeit.Uint8()),
		Away: uint(gofakeit.Uint8()),
	}

	applyOptions(&notification, options...)

	return notification
}
