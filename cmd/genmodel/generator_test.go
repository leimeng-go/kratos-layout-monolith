package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	sql := `CREATE TABLE users (
		id bigint NOT NULL AUTO_INCREMENT,
		username varchar(64) NOT NULL,
		password varchar(256) NOT NULL,
		email varchar(128) DEFAULT NULL,
		nickname varchar(64) DEFAULT NULL,
		avatar varchar(256) DEFAULT NULL,
		status int DEFAULT '1',
		created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		UNIQUE KEY idx_username (username),
		UNIQUE KEY idx_email (email)
	);`

	table, err := ParseSQL(sql)
	if err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	err = Generate(table, "data", "github.com/go-kratos/kratos-layout-monolith",
		"github.com/go-kratos/kratos-layout-monolith/internal/moduser/biz", outDir)
	if err != nil {
		t.Fatal(err)
	}

	// Check model file exists and contains struct
	modelPath := filepath.Join(outDir, "users_model_gen.go")
	modelContent, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(modelContent), "type User struct") {
		t.Error("model file should contain User struct")
	}
	if !strings.Contains(string(modelContent), `TableName() string`) {
		t.Error("model file should contain TableName method")
	}

	// Check cache repo file exists and contains methods
	repoPath := filepath.Join(outDir, "users_cache_gen.go")
	repoContent, err := os.ReadFile(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(repoContent)
	for _, method := range []string{
		"CreateUser", "UpdateUser", "GetUserByID",
		"GetUserByUsername", "GetUserByEmail",
		"ListUsers", "DeleteUser", "modelCacheKeys",
	} {
		if !strings.Contains(content, method) {
			t.Errorf("repo file should contain %s method", method)
		}
	}
	if !strings.Contains(content, "users:") {
		t.Error("repo file should contain cache prefix")
	}
}
