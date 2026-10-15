BEGIN;

CREATE TABLE IF NOT EXISTS provider_matches (
    id                bigserial               primary key,
    match_id          bigint                  not null unique,
    provider_id       bigint                  not null,
    provider_match_id varchar(64)             not null,
    starts_at         timestamptz             not null,
    home_score        smallint,
    away_score        smallint,
    provider_status   varchar(64),
    status            external_match_status   not null default 'not_started',
    created_at        timestamptz             not null default now(),
    updated_at        timestamptz             not null default now(),
    foreign key (match_id)    references matches   (id) on update cascade on delete cascade,
    foreign key (provider_id) references providers (id) on update cascade on delete restrict,
    unique (provider_id, provider_match_id)
);

CREATE TRIGGER provider_matches_update_timestamp
    BEFORE UPDATE ON provider_matches
    FOR EACH ROW EXECUTE PROCEDURE trigger_set_updated_at();

INSERT INTO provider_matches (match_id, provider_id, provider_match_id, starts_at, home_score, away_score, status)
SELECT em.match_id, p.id, em.id::text, m.starts_at, em.home_score, em.away_score, em.status
FROM external_matches em
JOIN matches m ON m.id = em.match_id
JOIN providers p ON p.name = 'fotmob';

COMMIT;
