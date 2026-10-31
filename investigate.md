## Open issues

When admin adds a match - provider returns outdated match start time (later than actual time). It is saved to provider_matches and matches.
As a consequence of that first check result task is happening later and the match result is added later than it should be.

example: match_id - 767: Epitsentr - FC Kharkiv
Actually starts 13:00 (GMT+3) / 10:00 (UTC)
In `provider_matches` starts_at is 2026-10-24 12:30 +00:00 / 15:30 (GMT+3)
In `matches` starts_at is 2026-10-24 12:30 +00:00 / 15:30 (GMT+3)
First scheduled result check is 2026-10-24 14:25 +00:00 / 17:25 (GMT+3)

Initially the football game was planned 15:30 (GMT+3) but later moved to 13:00 (GMT+3). 
Admin already knew the match starts 13:00 and set that time, and fotmob (provider) still had the old time when admin added the match.

After first attempt to check result:
`provider_matches` starts_at is 2026-10-24 10:00 +00:00 (updated to the actual match start time)
`matches` starts_at is 2026-10-24 12:30 +00:00 (the same)