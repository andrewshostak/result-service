## List of improvements
- [ ] functional tests
  - [ ] improvements
    - [ ] access parsed config values in tests: queue names, first attempt delay, interval
    - [ ] verify gcloud was reached
    - [ ] add cases for error response from gcloud tasks
    - [ ] Add match_id to subscription notified log
  - [ ] auth middleware
  - [ ] google middleware
- [ ] migrate from gorm to sqlc
- [ ] encode secret keys, do not store them in plain text
- [ ] configure CI in github
  - [ ] linter
  - [ ] deploy
- [ ] implement graceful shutdown
- [ ] investigate possible replacement zerolog with slog (if not - make gin and zerolog friends)
- [ ] find a proper way of handling secrets (remove secrets from `service.yaml`)
- [ ] check the context cancellation proper usage (defer cancel must be present)
- [ ] handle half-time status during the result check specially - next check more than +5 minutes
- [ ] add recover middleware

## Known issues
- [ ] matches postponed to tomorrow disappear from the search and become cancelled
- [ ] when result status fails then subscription status should be set to cancelled as well (not remain 'pending')

## Features

### 1) Check result task per provider match instead of per match
One `check_result_task` per provider per match instead of a single task per match that fans out to all providers in `ResultCheckerService.CheckResult` — needed once a second result provider is onboarded, to properly arbitrate providers disagreeing on a match's status instead of picking the first one found. Approach: `MatchService.Create` schedules a separate task for each provider that returned the match; 
`CheckResult` then receives a provider-scoped task, queries only that provider's API, and updates only that provider's `provider_match` row. 
This avoids the `ListByMatchID` + fan-out loop in `CheckResult`, keeps each task independently retryable per provider, and makes it trivial to add/remove a provider without changing the check logic. 
Trade-off: task scheduling becomes more complex in `Create` (one `ScheduleResultCheck` call per provider), and `check_result_tasks` needs a `provider_id` column to carry provider context into the task handler. 
Until this lands, `CheckResult` resolves disagreement between providers by picking the first provider match it finds a result for; with a single provider (fotmob) this is deterministic.