-- Rollback
ALTER TABLE users
RENAME COLUMN identity_doc_ref TO government_id_photo_id;