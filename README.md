# football-result-service

## General Info

The purpose of this service is to make the life of administrators of football sites easier. 
Instead of monitoring football matches and adding results manually, their apps can use the webhook to receive results automatically.
`result-service` is created for **prognoz** project ([web-app](https://github.com/andrewshostak/prognoz_web_app), [api](https://github.com/andrewshostak/prognoz_api)), but not restricted to only it. 
Feel free to use this service for your needs.

## Technical implementation

### Characters

(Integration with `prognoz` project as an example)

- Football Result Service / `result-service` - This service.
- External Results API (Fotmob) / `fotmob-api` - The source of the football matches results.
- Prognoz API Server / `prognoz-api` - The service that wants to receive the results.
- Google Cloud Tasks / `cloud-tasks` - The service that schedules tasks to check match results and notify subscribers.

### Data persistence

`result-service` has a **relational database**. It is visually represented below:

```mermaid
erDiagram
    Team {
        Int id PK
    }
    
    Alias {
        Int id PK
        Int team_id FK
        String alias UK 
    }
    
    Match {
        Int id PK
        Int home_team_id FK
        Int away_team_id FK
        Date started_at
        String result_status
    }
    
    ExternalMatch {
        Int id PK
        Int match_id FK
        Int home_score
        Int away_score
        Json data
    }
    
    Subscription {
        Int id PK
        String url UK
        Int match_id FK
        String key
        Date created_at
        String subscription_status
        Date notified_at
        String error
    }
    
    ExternalTeam {
        Int id PK
        Int team_id FK
    }
    
    CheckResultTask {
        Int id PK
        Int match_id FK
        String name UK
        Int attempt_number
    }
    
    Team ||--o{ Alias : has 
    Team ||--o{ Match : has
    Match ||--|| ExternalMatch : has
    Match ||--o{ Subscription : has
    Team ||--|| ExternalTeam : has
    Match ||--|| CheckResultTask : has
```

Table names are pluralized. The tables `teams`, `aliases`, `external-teams` are pre-filled with the data of `fotmob-api`.

#### Description of possible match `result_status` values:

| `result_status`    | Description                                                                                                                                           |
|--------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------|
| `not_scheduled`    | Match is created, but fixture creation or task scheduling fails                                                                                       |
| `scheduled`        | Match is created and a task is scheduled. If there was an attempt to get a result but a match was not ended the status `scheduled` remains unchanged. |
| `scheduling_error` | An attempt to reschedule task was unsuccessful.                                                                                                       |
| `received`         | Match result is received.                                                                                                                             |
| `api_error`        | Request to fotmob-api to get match result was unsuccessful.                                                                                           |
| `cancelled`        | Received a status from fotmob-api indicates that match was canceled. No new task is rescheduled.                                                      |

#### Description of possible subscription `subscription_status` values:

| `subscription_status` | Description                                                          |
|-----------------------|----------------------------------------------------------------------|
| `pending`             | Subscription is created, but match result is not yet received.       |
| `scheduling_error`    | Attempt to create a task was unsuccessful.                           |
| `successful`          | Subscriber successfully notified. Column `notified_at` gets a value. |
| `subscriber_error`    | Subscriber returned an error. Column `error` gets a value.           |

## Flow diagrams

### Overall

```mermaid
flowchart TD
    subgraph Group 1
        A[Create a match] --> B[Subscribe on result receiving]
    end

    subgraph Group 2
        C[Receive trigger to check match result] --> D[Notify subscribers]
    end

    E[Delete a subscription]

    B --> C
    B --> E
```

### Create a match

```mermaid
sequenceDiagram
participant API as prognoz-api
participant ResultService as result-service
participant Fotmob as fotmob-api
participant CloudTasks as cloud-tasks
API->>ResultService: Sends a request to create a match
Activate ResultService
ResultService->>ResultService: Gets team ids by aliases from the DB
ResultService->>ResultService: Gets match by team ids and starting time from the DB
alt match is found and result status is scheduled
    ResultService-->>API: Returns match response
end
ResultService->>+Fotmob: Sends a request with date & timezone
Fotmob-->>-ResultService: Returns all matches for the date
ResultService->>ResultService: Finds a match in the response by aliases and starting time
ResultService->>ResultService: Saves match with status not_scheduled and external match to the DB
ResultService->>CloudTasks: Creates a task to check result with schedule-time (starting time + 115 minutes)
Activate CloudTasks
CloudTasks-->>ResultService: Returns task id
Deactivate CloudTasks
ResultService->>ResultService: Saves check result task to the DB
ResultService->>ResultService: Updates match status to scheduled
ResultService-->>API: Returns match response
Deactivate ResultService
```

### Subscribe on result receiving

```mermaid
sequenceDiagram
participant API as prognoz-api
participant ResultService as result-service
API->>ResultService: Sends a request to create subscription
Activate ResultService
ResultService->>ResultService: Gets match from the DB
alt Match result status in not scheduled
ResultService-->>API: Returns error
end
ResultService->>ResultService: Saves subscription to the DB
ResultService-->>API: Returns success
Deactivate ResultService
```

### Receive trigger to check match result

```mermaid
sequenceDiagram
    participant CloudTasks as cloud-tasks
    participant ResultService as result-service
    participant Fotmob as fotmob-api
    CloudTasks->>+ResultService: Sends a request to check match result
    ResultService->>+Fotmob: Sends a request to get match details
    Fotmob-->>-ResultService: Returns a match
    ResultService->>ResultService: Updates external match data
    alt match is not yet ended
        ResultService->>CloudTasks: Creates a new task to check result with backoff
        ResultService-->>CloudTasks: Returns success
    end
    ResultService->>ResultService: Gets all subscriptions of the match from the DB 
    loop Each subscription
        ResultService->>CloudTasks: Create a task to notify subscriber
    end
    ResultService-->>-CloudTasks: Returns success
```

### Notify subscribers

```mermaid
sequenceDiagram
    participant CloudTasks as cloud-tasks
    participant ResultService as result-service
    participant API as prognoz-api
    CloudTasks->>+ResultService: Sends a request to notify subscriber
    ResultService->>ResultService: Gets a subscription and a match from the DB
    ResultService->>+API: Sends a request with the match result
    API-->>-ResultService: Returns success
    ResultService->>ResultService: Updates subscription status
    ResultService-->>-CloudTasks: Returns success
    
```

### Delete a subscription

```mermaid
sequenceDiagram
participant API as prognoz-api
participant ResultService as result-service
participant CloudTasks as cloud-tasks
API->>ResultService: Sends a request to remove subscription
Activate ResultService
ResultService->>ResultService: Deletes subscription from DB
alt other subscriptions for this match exist
ResultService-->>API: Returns success
end
ResultService->>CloudTasks: Sends a request to remove scheduled task
Activate CloudTasks
CloudTasks-->>ResultService: Returns success
Deactivate CloudTasks
ResultService->>ResultService: Removes match and fixture from DB
ResultService-->>API: Returns success
Deactivate ResultService
```

### Authorization

`prognoz-api` => `result-service`
1) A secret key is generated, hashed and set to env variables
2) `prognoz-api` attaches secret key to requests to `result-service`
3) `result-service` has a middleware that checks presence and validity of secret-key

`result-service` => `prognoz-api`
1) When `prognoz-api` creates a subscription it sends a secret-key
2) Secret-key is saved in `subscriptions` table for each subscription  
3) When `result-service` calls subscription `url` it attaches secret-key to the request

### Back-fill aliases data

To back-fill aliases data a separate command is created. The command description:
- Accepts dates as a parameter
- Command has predefined list of league and country names (for example: Premier League - Ukraine, La Liga - Spain, etc.)
- For each date param calls `fotmob-api`s `matches` endpoint
- Extracts team names from the matches list
- For each team the command does the next actions in database 
  - checks if `alias` already exists
  - if not, creates a `team`, `alias`, `external_team` in transaction

### Implementation TODO
- [X] Connect to supabase from datagrip
- [X] Update database migrations: include tasks table & new statuses 
- [X] Add a command to run migrations
- [X] Run migrations on supabase from locally-running command
- [X] Remove scheduler related code
- [X] Configure google cloud run: create project, region, cloud run service
- [X] Deploy service to cloud run
- [X] Configure cloud run settings
- [X] Configure google cloud tasks: region, two queues
- [X] Start service locally with launching cloud task client
- [X] Modify existing POST /matches
- [X] Modify existing POST /subscriptions
- [X] Implement client methods to interact with google cloud tasks API
    - [X] Create a new check-result task
    - [X] Create a new notify-subscriber task
    - [X] Remove check-result task
- [X] Verify match creation flow works
- [X] Modify existing DELETE /subscriptions
- [X] Verify subscription deletion flow works
- [X] Create a new endpoint to be called by cloud task for checking match result
- [X] Create a new endpoint to be called by cloud task for notifying subscriber
- [ ] Migrate to fotmob API
  - [ ] Modify backfill aliases command
    - [X] Create client method to call fotmob matches list (by date)
    - [X] Update command logic to accept date, update leagues, update mapping
  - [ ] Modify Match creation flow 
    - [ ] Update football_api_fixtures table: rename to external_matches, add score_home, score_away
    - [ ] Create client method to call fotmob match details (by id)
    - [ ] Update match creation endpoint
    - [ ] Delete football_api_team table and its references
- [ ] Include signed requests & validate google-auth middleware
- [ ] Find a solution for same-time results (i.e. do not process tasks concurrently)
- [ ] Add created_at / updated_at columns

// TODO: use case what happens if match sent starting time is different from match football api starting time
