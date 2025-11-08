-- Rename the column to be more generic, as it will hold a message_id
ALTER TABLE users
RENAME COLUMN government_id_photo_id TO identity_doc_ref;