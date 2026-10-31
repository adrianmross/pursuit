"""Print the Neon create-anonymized-branch request for a PII-free preview DB.

Usage: python3 scripts/preview-masking-rules.py PARENT_BRANCH_ID [NAME] > body.json
then POST it to /api/v2/projects/{project_id}/branch_anonymized.
See "Preview database" in README.md.
"""
import json
import sys

UUID = "anon.dummy_uuidv4()"
NAME = "pg_catalog.concat(anon.fake_first_name(), ' ', anon.fake_last_name())"
EMAIL = "pg_catalog.concat(anon.dummy_uuidv4(), '@example.com')"
URL = "pg_catalog.concat('https://example.com/', anon.dummy_uuidv4())"
WORDS = "anon.lorem_ipsum(words := 6)"
PARA = "anon.lorem_ipsum(words := 30)"
# companies_user_name_lower is unique per user, so suffix a short random tag.
COMPANY = "pg_catalog.concat(anon.fake_company(), ' ', pg_catalog.left(anon.dummy_uuidv4()::text, 4))"

fn = {  # (schema, table, column): masking_function
    ("public", "users", "name"): NAME,
    ("public", "users", "email"): EMAIL,
    ("public", "companies", "name"): COMPANY,
    ("public", "companies", "website"): URL,
    ("public", "companies", "notes"): PARA,
    ("public", "company_caps", "company"): UUID,  # unique with user_id
    ("public", "roles", "posting_url"): URL,
    ("public", "roles", "location"): "anon.fake_city()",
    ("public", "roles", "salary_text"): WORDS,
    ("public", "applications", "notes"): PARA,
    ("public", "applications", "next_step"): WORDS,
    ("public", "applications", "contact_email"): EMAIL,
    ("public", "events", "title"): WORDS,
    ("public", "events", "body"): PARA,
    ("public", "events", "actor_name"): NAME,
    ("public", "approvals", "note"): WORDS,
    ("public", "approvals", "actor_name"): NAME,
    ("public", "audit_log", "summary"): WORDS,
    ("public", "audit_log", "actor_name"): NAME,
    ("public", "interviews", "interviewer"): NAME,
    ("public", "interviews", "prep"): PARA,
    ("public", "interviews", "notes"): PARA,
    ("public", "tasks", "title"): WORDS,
    ("public", "tasks", "notes"): PARA,
    ("public", "questions", "prompt"): WORDS,
    ("public", "questions", "answer"): WORDS,
    ("public", "standing_answers", "answer"): WORDS,
    ("public", "custom_field_values", "value"): WORDS,
    ("public", "posting_checks", "detail"): WORDS,
    ("public", "api_tokens", "token_hash"): UUID,  # prod tokens must not work here
    ("public", "oauth_grants", "code_hash"): UUID,
    ("public", "webhooks", "secret"): UUID,
    ("public", "webhooks", "url"): URL,
    ("neon_auth", "user", "name"): NAME,
    ("neon_auth", "user", "email"): EMAIL,
    ("neon_auth", "session", "token"): UUID,
    ("neon_auth", "verification", "value"): UUID,
    ("neon_auth", "invitation", "email"): EMAIL,
}
val = {  # (schema, table, column): masking_value
    ("public", "users", "avatar"): "''",
    ("public", "companies", "icon_url"): "''",
    ("public", "documents", "body"): "''",  # real résumé
    ("public", "documents", "filename"): "'document.pdf'",
    ("public", "artifacts", "body"): "''",
    ("public", "artifacts", "filename"): "'artifact.pdf'",
    ("public", "audit_log", "metadata"): "'{}'",
    ("public", "idempotency_keys", "response_body"): "'{}'",
    ("public", "applications", "resume_file_ref"): "''",
    ("public", "applications", "resume_version"): "''",
    # Production's Neon Auth signing key must never sit in a preview copy.
    ("neon_auth", "jwks", "privateKey"): "'masked'",
    ("neon_auth", "jwks", "publicKey"): "'masked'",
    ("neon_auth", "project_config", "social_providers"): "'[]'",
    ("neon_auth", "project_config", "email_provider"): "NULL",
    ("neon_auth", "project_config", "webhook_config"): "NULL",
    ("public", "oauth_grants", "user_code_hash"): "NULL",
    ("neon_auth", "user", "image"): "NULL",
    ("neon_auth", "session", "ipAddress"): "NULL",
    ("neon_auth", "session", "userAgent"): "NULL",
    ("neon_auth", "account", "accessToken"): "NULL",
    ("neon_auth", "account", "refreshToken"): "NULL",
    ("neon_auth", "account", "idToken"): "NULL",
    ("neon_auth", "account", "password"): "NULL",
}

rules = [dict(database_name="neondb", schema_name=s, table_name=t, column_name=c, masking_function=f) for (s, t, c), f in fn.items()]
rules += [dict(database_name="neondb", schema_name=s, table_name=t, column_name=c, masking_value=v) for (s, t, c), v in val.items()]
body = {
    "branch_create": {"branch": {"name": sys.argv[2] if len(sys.argv) > 2 else "preview-anon", "parent_id": sys.argv[1]}, "endpoints": [{"type": "read_write"}]},
    "masking_rules": rules,
    "start_anonymization": True,
}
print(json.dumps(body))
