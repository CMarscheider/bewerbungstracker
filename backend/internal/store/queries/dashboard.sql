-- name: ListOpenDeadlines :many
SELECT a.id AS application_id, c.name AS company_name, a.position_title,
       le.type AS event_type, le.due_on
FROM latest_events le
JOIN applications a ON a.id = le.application_id
JOIN companies c ON c.id = a.company_id
WHERE le.due_on IS NOT NULL
  AND le.due_on <= sqlc.arg('until_date')::date
ORDER BY le.due_on, c.name, a.id;

-- name: ListUpcomingAppointments :many
SELECT a.id AS application_id, c.name AS company_name, a.position_title,
       le.type AS event_type, le.occurred_on, le.interview_round
FROM latest_events le
JOIN applications a ON a.id = le.application_id
JOIN companies c ON c.id = a.company_id
WHERE le.type IN ('ScreeningGespraech', 'Interview', 'Kennenlerntag')
  AND le.occurred_on BETWEEN sqlc.arg('from_date')::date AND sqlc.arg('until_date')::date
ORDER BY le.occurred_on, c.name, a.id;

-- name: ListAllEventsWithSource :many
SELECT e.application_id, a.source, e.type, e.occurred_on, e.created_at
FROM application_events e
JOIN applications a ON a.id = e.application_id
ORDER BY e.application_id, e.created_at, e.id;
