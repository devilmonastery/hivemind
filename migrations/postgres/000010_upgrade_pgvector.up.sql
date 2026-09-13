-- Keep the installed extension aligned with the pgvector version in the
-- postgres-gembed image after an image upgrade.
ALTER EXTENSION vector UPDATE;
