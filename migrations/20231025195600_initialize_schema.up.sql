begin;

create table if not exists teams (
    id bigserial primary key
);

create table if not exists external_teams (
    id bigserial primary key,
    team_id bigint not null unique,
    foreign key (team_id) references teams (id) on update cascade on delete restrict
);

create type result_status as enum ('not_scheduled', 'scheduled', 'scheduling_error', 'received', 'api_error', 'cancelled');

create table if not exists matches (
    id bigserial primary key,
    home_team_id bigint not null,
    away_team_id bigint not null,
    starts_at timestamptz not null,
    result_status result_status not null default 'not_scheduled',
    foreign key (home_team_id) references teams (id) on update cascade on delete restrict,
    foreign key (away_team_id) references teams(id) on update cascade on delete restrict,
    unique (home_team_id,away_team_id,starts_at)
);

create table if not exists aliases (
    id bigserial primary key,
    team_id bigint not null,
    alias varchar(64) not null unique,
    foreign key (team_id) references teams (id) on update cascade on delete restrict
);

create type subscription_status as enum ('pending', 'scheduling_error', 'successful', 'subscriber_error');

create table if not exists subscriptions (
    id bigserial primary key,
    url text not null unique,
    match_id bigint not null,
    key text not null,
    status subscription_status not null default 'pending',
    error text, -- TODO: rename to subscriber_error
    notified_at timestamptz,
    created_at timestamptz not null default now(),
    foreign key (match_id) references matches (id) on update cascade on delete cascade
);

create type external_match_status as enum ('not_started', 'cancelled', 'in_progress', 'finished', 'unknown');

create table if not exists external_matches
(
    id bigserial primary key,
    match_id bigint not null unique,
    home_score smallint,
    away_score smallint,
    status external_match_status not null default 'not_started',
    foreign key (match_id) references matches (id) on update cascade on delete cascade
);

create table if not exists check_result_tasks
(
    id bigserial primary key,
    match_id bigint not null unique,
    name text not null unique,
    attempt_number integer not null default 1,
    execute_at timestamptz not null,
    created_at timestamptz not null default now(),
    foreign key (match_id) references matches (id) on update cascade on delete cascade
);

commit;