-- name: ListCompanies :many
SELECT c.id, c.name, c.website, c.notes, c.created_at,
       count(a.id)::int AS application_count
FROM companies c
LEFT JOIN applications a ON a.company_id = c.id
GROUP BY c.id
ORDER BY lower(c.name);

-- name: GetCompany :one
SELECT c.id, c.name, c.website, c.notes, c.created_at,
       count(a.id)::int AS application_count
FROM companies c
LEFT JOIN applications a ON a.company_id = c.id
WHERE c.id = $1
GROUP BY c.id;

-- name: CreateCompany :one
INSERT INTO companies (name, website, notes)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateCompany :one
UPDATE companies
SET name = $2, website = $3, notes = $4
WHERE id = $1
RETURNING *;

-- name: DeleteCompany :execrows
DELETE FROM companies WHERE id = $1;

-- name: GetCompanyByName :one
SELECT * FROM companies WHERE lower(name) = lower($1);
