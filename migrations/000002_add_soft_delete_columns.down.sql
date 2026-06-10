ALTER TABLE users DROP INDEX idx_del_state;
ALTER TABLE users DROP COLUMN deleted_at;
ALTER TABLE users DROP COLUMN del_state;
