BEGIN;

CREATE TABLE IF NOT EXISTS external_teams (
    id bigserial primary key,
    team_id bigint not null unique,
    foreign key (team_id) references teams (id) on update cascade on delete restrict
);

CREATE TABLE IF NOT EXISTS external_matches (
    id bigserial primary key,
    match_id bigint not null unique,
    home_score smallint,
    away_score smallint,
    status external_match_status not null default 'not_started',
    foreign key (match_id) references matches (id) on update cascade on delete cascade
);

COMMIT;
