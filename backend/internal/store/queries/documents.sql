-- name: SetDocumentsState :exec
UPDATE applications SET documents_state = $2, documents_error = $3, updated_at = now() WHERE id = $1;

-- name: SetDraftCreated :exec
UPDATE applications SET documents_state = 'entwurf_angelegt', documents_error = NULL, gmail_draft_at = now(), updated_at = now()
WHERE id = $1;

-- name: GetDocuments :one
SELECT * FROM application_documents WHERE application_id = $1;

-- name: UpsertDocuments :one
INSERT INTO application_documents (application_id, version, language, cover_letter, profile_line, highlights,
                                   mail_subject, mail_body, pdf, file_name, rendered_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
ON CONFLICT (application_id) DO UPDATE
SET version = EXCLUDED.version, language = EXCLUDED.language, cover_letter = EXCLUDED.cover_letter,
    profile_line = EXCLUDED.profile_line, highlights = EXCLUDED.highlights, mail_subject = EXCLUDED.mail_subject,
    mail_body = EXCLUDED.mail_body, pdf = EXCLUDED.pdf, file_name = EXCLUDED.file_name,
    rendered_at = EXCLUDED.rendered_at, updated_at = now()
RETURNING *;

-- name: ListAgentApplicationsByDocumentsState :many
SELECT a.id, c.name AS company_name, c.website AS company_website, a.position_title, a.job_url, a.location,
       a.contact_email, a.posting_text, a.fit_reason, a.documents_state,
       COALESCE(d.version, 0)::int AS documents_version
FROM applications a
JOIN companies c ON c.id = a.company_id
LEFT JOIN application_documents d ON d.application_id = a.id
WHERE a.documents_state = $1
ORDER BY a.updated_at, a.id;
