UPDATE feeds SET folder_id = NULL WHERE folder_id IS NOT NULL;
DROP INDEX IF EXISTS idx_folders_name;
DROP TABLE IF EXISTS folders;
