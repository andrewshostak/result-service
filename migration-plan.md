# Migration Plan

## Principles

- Every PR is independently deployable and backward compatible with the current DB state at the time it lands.
- DB migrations are always run **before** the code that depends on them is deployed.
- Because the service runs on App Engine and sleeps between matches, the window where old code runs against a partially-migrated DB is practically zero — but the plan is designed to be safe even if it weren't.
- The general pattern for replacing a table: **add new table → backfill + dual-write in the same PR → switch reads → drop old table**.
- Backfill and dual-write must land in the same PR to avoid a data gap: any rows written to the old table between the migration and the code deployment would be missing from the new table.

---

## Phase 1 — Foundation (pure DB, no behavior change)

### PR 1 — Add `providers` table

**Migration:**
- Create `providers` table (`id`, `name`, `created_at`, `updated_at`)
- Insert one row: `fotmob`

**Code:** none.

**Why safe:** new table, nothing references it yet. Old code is completely unaffected.

---

### PR 2 — Add timestamps to all existing tables

**Migration:**
- Add `created_at` and `updated_at` (with `DEFAULT NOW()`) to: `teams`, `aliases`, `matches`
- Add `updated_at` (with `DEFAULT NOW()`) to: `subscriptions`, `check_result_tasks`
- Existing rows get the migration timestamp — not historically accurate but acceptable

**Code:**
- Add `CreatedAt` / `UpdatedAt` fields to the relevant gorm models

**Why safe:** adding columns with defaults never breaks existing queries or reads. Gorm ignores unknown columns, so deploying the migration before the code is fine.

---

## Phase 2 — New tables (parallel structures, backfill + dual-write together)

### PR 3 — Add `provider_teams` table

**Migration:**
- Create `provider_teams` table (`id`, `team_id`, `provider_id`, `provider_team_id` as string, `created_at`, `updated_at`)
- Backfill: for every row in `external_teams`, insert a corresponding row into `provider_teams` using the fotmob `provider_id` and casting `external_teams.id` to string as `provider_team_id`

**Code:**
- Add `ProviderTeamRepository` that writes to `provider_teams`
- Wherever `external_teams` is written to (backfill command), also write to `provider_teams`

**Why safe:** new table. Backfill covers all historical data. Dual-write ensures no rows are missed from this point on. Old code reads are unchanged.

---

### PR 4 — Add `provider_matches` table

**Migration:**
- Create `provider_matches` table (`id`, `match_id`, `provider_id`, `provider_match_id` as string, `starts_at`, `home_score`, `away_score`, `status`, `provider_status`, `created_at`, `updated_at`)
- Backfill: for every row in `external_matches`, insert into `provider_matches` using fotmob `provider_id`, casting `external_matches.id` to string as `provider_match_id`, and copying `starts_at` from the corresponding `matches` row

**Code:**
- Add `ProviderMatchRepository` that writes to `provider_matches`
- In `ResultCheckerService`, after saving to `external_matches`, also save to `provider_matches` (with `provider_id` from the active provider's `Name()`)
- Reads still come from `external_matches`

**Why safe:** backfill covers all historical data. Dual-write is deployed immediately after, so new match result data lands in both tables from this point on. Old reads are unchanged.

---

## Phase 3 — Code migration

### PR 5 — `ResultProvider` interface + slice injection

**Migration:** none.

**Code:**
- Rename `ExternalAPIClient` to `ResultProvider` in `match/contract.go`, add `Name() string` to the interface
- Add `Name() string` to `FotmobClient`
- Change `MatchService` and `ResultCheckerService` to hold `[]ResultProvider` instead of a single client
- Extract `findMatchAcrossProviders` in `MatchService` and `findMatchResultAcrossProviders` in `ResultCheckerService`
- In `cmd/server/main.go`: build `providers := []match.ResultProvider{fotmobClient}` and inject the slice

**Why safe:** pure refactor — behavior is identical with a single-element slice. No DB changes, no data touched.

---

### PR 6 — Switch alias lookup to use `provider_teams`

**Migration:** none (table already exists and is kept in sync from PR 3).

**Code:**
- Add `FindByTeamAndProvider(ctx, teamID, providerName)` to `ProviderTeamRepository`
- Split `findAlias` in `MatchService` into two steps: alias → `team_id`, then `ProviderTeamRepository` → `provider_team_id`
- `Alias` domain model no longer carries `ExternalTeam`
- Remove `ExternalTeam` join from `AliasRepository.Find`

**Why safe:** `provider_teams` has been backfilled and kept in sync since PR 3. Reads switch from `external_teams` to `provider_teams` — same data, new table.

---

## Phase 4 — Switch reads, stop writing to old tables

### PR 7 — Switch all reads to `provider_matches`, remove `external_matches` writes

**Migration:** none.

**Code:**
- Update `MatchRepository.One` to join `provider_matches` instead of `external_matches`
- Update `SubscriberNotifierService` to read scores from `provider_matches` (filtered by provider)
- Remove the `ExternalMatchRepository` write from `ResultCheckerService` (keep only the `ProviderMatchRepository` write introduced in PR 4)

**Why safe:** dual-write has been running since PR 4, so `provider_matches` is fully up to date. Reads switch to the new table only after the data is guaranteed to be there.

---

## Phase 5 — Cleanup

### PR 8 — Drop `external_teams` and `external_matches`

**Migration:**
- Drop `external_matches`
- Drop `external_teams`

**Code:**
- Remove `ExternalMatchRepository` and `ExternalTeam` gorm model entirely
- Remove any remaining references

**Why safe:** no code references either table after PRs 6 and 7. Run migration after code is deployed and verified stable.

---

## Summary

| PR | Type | Depends on |
|---|---|---|
| PR 1 — `providers` table | DB only | — |
| PR 2 — timestamps | DB + gorm models | PR 1 |
| PR 3 — `provider_teams` + backfill + dual-write | DB + code | PR 1 |
| PR 4 — `provider_matches` + backfill + dual-write | DB + code | PR 1, PR 5* |
| PR 5 — `ResultProvider` interface | Code only | — |
| PR 6 — switch alias reads to `provider_teams` | Code only | PR 3 |
| PR 7 — switch result reads to `provider_matches` | Code only | PR 4 |
| PR 8 — drop old tables | DB + code cleanup | PR 6, PR 7 |

*PR 4 needs `provider.Name()` to write the provider identifier when dual-writing, so PR 5 should land first.
