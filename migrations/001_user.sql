CREATE TABLE users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(64) NOT NULL,
    password VARCHAR(256) NOT NULL COMMENT '密码哈希',
    email VARCHAR(128),
    phone VARCHAR(32),
    nickname VARCHAR(64),
    avatar VARCHAR(256),
    status INT NOT NULL DEFAULT 1 COMMENT '状态: 0=禁用, 1=正常',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY idx_username (username),
    UNIQUE KEY idx_email (email),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';
