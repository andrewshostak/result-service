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
- [X] implement graceful shutdown
- [ ] investigate possible replacement zerolog with slog (if not - make gin and zerolog friends)
- [ ] find a proper way of handling secrets (remove secrets from `service.yaml`)
- [ ] check the context cancellation proper usage (defer cancel must be present)
- [ ] handle half-time status during the result check specially - next check more than +5 minutes
- [ ] add recover middleware
- [ ] add error middleware (+remove logs from app layers)
- [ ] configure connection pool
- [ ] add healthcheck to docker compose (DB)

## Known issues
- [ ] matches postponed to tomorrow disappear from the search and become cancelled
- [ ] when result status fails then subscription status should be set to cancelled as well (not remain 'pending')
