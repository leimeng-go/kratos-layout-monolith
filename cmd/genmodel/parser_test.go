package main

import (
	"testing"
)

func TestParseCreateTable(t *testing.T) {
	sql := `CREATE TABLE users (
		id bigint NOT NULL AUTO_INCREMENT,
		username varchar(64) NOT NULL,
		password varchar(256) NOT NULL,
		email varchar(128) DEFAULT NULL,
		phone varchar(32) DEFAULT NULL,
		nickname varchar(64) DEFAULT NULL,
		avatar varchar(256) DEFAULT NULL,
		status int DEFAULT '1',
		created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		UNIQUE KEY idx_username (username),
		UNIQUE KEY idx_email (email),
		KEY idx_phone (phone)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`

	table, err := ParseSQL(sql)
	if err != nil {
		t.Fatal(err)
	}

	if table.Name != "users" {
		t.Errorf("table name: want users, got %s", table.Name)
	}
	if table.GoName != "User" {
		t.Errorf("go name: want User, got %s", table.GoName)
	}
	if table.PrimaryKey.Name != "id" {
		t.Errorf("primary key: want id, got %s", table.PrimaryKey.Name)
	}
	if table.PrimaryKey.GoType != "int64" {
		t.Errorf("pk type: want int64, got %s", table.PrimaryKey.GoType)
	}
	if len(table.Columns) != 10 {
		t.Errorf("columns: want 10, got %d", len(table.Columns))
	}
	if len(table.UniqueIndexes) != 2 {
		t.Errorf("unique indexes: want 2, got %d", len(table.UniqueIndexes))
	}
	foundUsername := false
	foundEmail := false
	for _, idx := range table.UniqueIndexes {
		if idx.Name == "idx_username" {
			foundUsername = true
			if idx.ColumnName != "username" {
				t.Errorf("idx_username column: want username, got %s", idx.ColumnName)
			}
		}
		if idx.Name == "idx_email" {
			foundEmail = true
		}
	}
	if !foundUsername {
		t.Error("missing idx_username")
	}
	if !foundEmail {
		t.Error("missing idx_email")
	}
}

func TestParseCreateTableNoUniqueIndex(t *testing.T) {
	sql := `CREATE TABLE roles (
		id int NOT NULL AUTO_INCREMENT,
		name varchar(64) NOT NULL,
		PRIMARY KEY (id)
	);`

	table, err := ParseSQL(sql)
	if err != nil {
		t.Fatal(err)
	}
	if len(table.UniqueIndexes) != 0 {
		t.Errorf("unique indexes: want 0, got %d", len(table.UniqueIndexes))
	}
	if table.PrimaryKey.Name != "id" {
		t.Errorf("pk: want id, got %s", table.PrimaryKey.Name)
	}
}

func TestToCamel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user_name", "UserName"},
		{"id", "Id"},
		{"created_at", "CreatedAt"},
		{"email", "Email"},
	}
	for _, tt := range tests {
		if got := toCamel(tt.input); got != tt.expected {
			t.Errorf("toCamel(%q): want %q, got %q", tt.input, tt.expected, got)
		}
	}
}
