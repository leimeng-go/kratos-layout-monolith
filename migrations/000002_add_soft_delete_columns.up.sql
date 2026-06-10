ALTER TABLE users ADD COLUMN del_state INT NOT NULL DEFAULT 0 COMMENT '软删除: 0=未删除, 1=已删除' AFTER status;
ALTER TABLE users ADD COLUMN deleted_at TIMESTAMP NULL DEFAULT NULL COMMENT '删除时间' AFTER del_state;
ALTER TABLE users ADD INDEX idx_del_state (del_state);
