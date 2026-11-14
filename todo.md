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
- [ ] configure connection pool 

## Known issues
- [ ] matches postponed to tomorrow disappear from the search and become cancelled
- [ ] when result status fails then subscription status should be set to cancelled as well (not remain 'pending')

- [ ] failed check result re-scheduling, log:
  - 23:45 500 to /v1/triggers/result_check
  - 23:45 failed to create result-check task: rpc error: code = Unavailable desc = error reading from server: read tcp [fddf:3978:feb1:d745::c001]:23461->[2001:4860:4847:400::]:443: read: connection reset by peer
  - 23:46 result check skipped: unexpected result status scheduling_error
Issues:  
 - why does re-scheduling failed
 - failed rescheduling should retry
 - provider_match should not be frozen with in_progress
