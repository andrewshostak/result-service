begin;

create table if not exists teams (
    id bigserial primary key
);

create table if not exists football_api_teams (
    id bigserial primary key,
    team_id bigserial unique,
    foreign key (team_id) references teams (id) on update cascade on delete restrict
);

create type result_status as enum ('not_scheduled', 'scheduled', 'scheduling_error', 'received', 'api_error', 'cancelled');

create table if not exists matches (
    id bigserial primary key,
    home_team_id bigserial,
    away_team_id bigserial,
    starts_at timestamp not null,
    result_status result_status not null default 'not_scheduled',
    foreign key (home_team_id) references teams (id) on update cascade on delete restrict,
    foreign key (away_team_id) references teams(id) on update cascade on delete restrict,
    unique (home_team_id,away_team_id,starts_at)
);

create table if not exists aliases (
    id bigserial primary key,
    team_id bigserial,
    alias varchar(64) not null unique,
    foreign key (team_id) references teams (id) on update cascade on delete restrict
);

create type subscription_status as enum ('pending', 'scheduling_error', 'successful', 'subscriber_error');

create table if not exists subscriptions (
    id bigserial primary key,
    url text unique,
    match_id bigserial,
    key text,
    created_at timestamp not null,
    status subscription_status not null default 'pending',
    notified_at timestamp,
    error text,
    foreign key (match_id) references matches (id) on update cascade on delete cascade
);

create table if not exists football_api_fixtures
(
    id bigserial primary key,
    match_id bigserial unique,
    data jsonb not null,
    foreign key (match_id) references matches (id) on update cascade on delete cascade
);

create table if not exists check_result_tasks
(
    name text primary key,
    match_id bigserial unique,
    foreign key (match_id) references matches (id) on update cascade on delete cascade
);

commit;