begin;

drop table if exists external_matches;
drop table if exists subscriptions;
drop table if exists aliases;
drop table if exists check_result_tasks;
drop table if exists matches;
drop table if exists external_teams;
drop table if exists teams;
drop type result_status;
drop type subscription_status;
drop type external_match_status;

commit;