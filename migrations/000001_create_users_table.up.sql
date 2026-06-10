CREATE TABLE users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(64) NOT NULL UNIQUE COMMENT '用户名',
    password VARCHAR(256) NOT NULL COMMENT '密码哈希',
    email VARCHAR(128) UNIQUE COMMENT '邮箱',
    phone VARCHAR(32) COMMENT '手机号',
    nickname VARCHAR(64) COMMENT '昵称',
    avatar VARCHAR(256) COMMENT '头像URL',
    status INT NOT NULL DEFAULT 1 COMMENT '状态: 0=禁用, 1=正常',
    del_state INT NOT NULL DEFAULT 0 COMMENT '软删除: 0=未删除, 1=已删除',
    deleted_at TIMESTAMP NULL DEFAULT NULL COMMENT '删除时间',
    version BIGINT NOT NULL DEFAULT 0 COMMENT '乐观锁版本号',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_username (username),
    INDEX idx_email (email),
    INDEX idx_status (status),
    INDEX idx_del_state (del_state)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';
