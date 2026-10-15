BEGIN;

CREATE TABLE IF NOT EXISTS provider_teams (
    id                bigserial    primary key,
    team_id           bigint       not null,
    provider_id       bigint       not null,
    provider_team_id  varchar(64)  not null,
    created_at        timestamptz  not null default now(),
    updated_at        timestamptz  not null default now(),
    foreign key (team_id)     references teams     (id) on update cascade on delete restrict,
    foreign key (provider_id) references providers (id) on update cascade on delete restrict,
    unique (provider_id, provider_team_id),
    unique (team_id, provider_id)
);

CREATE TRIGGER provider_teams_update_timestamp
    BEFORE UPDATE ON provider_teams
    FOR EACH ROW EXECUTE PROCEDURE trigger_set_updated_at();

INSERT INTO provider_teams (team_id, provider_id, provider_team_id)
SELECT et.team_id, p.id, et.id::text
FROM external_teams et
JOIN providers p ON p.name = 'fotmob';

COMMIT;
