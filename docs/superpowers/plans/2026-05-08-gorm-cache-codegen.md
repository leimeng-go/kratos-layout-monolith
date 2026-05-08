# GORM 缓存层 + 代码生成器 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 GORM 构建自建缓存层（细粒度缓存、singleflight、空值缓存防穿透、随机 TTL 防雪崩、写前查询+主动失效），并从 SQL DDL 自动生成带缓存的 CRUD 代码，最终替换 user 模块。

**Architecture:** 在 `internal/pkg/cache` 中构建 `Take`/`Set`/`Del` 缓存抽象，用 `singleflight` 防击穿。`cmd/genmodel` 从 SQL DDL 解析表结构，用 Go string builder 生成 GORM Model + 带缓存的 Repo。最终将 `internal/moduser/data/user.go` 拆分为生成代码+手写代码分离的形式。biz 层完全不感知缓存。

**Tech Stack:** Go, GORM, go-redis, golang.org/x/sync/singleflight, alicebob/miniredis (test)

---

### Task 1: 添加 singleflight 依赖

**Files:**
- Modify: `go.mod` (auto), `go.sum` (auto)

- [ ] **Step 1: Run go get**

```bash
go get golang.org/x/sync/singleflight
```

- [ ] **Step 2: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add golang.org/x/sync/singleflight dependency"
```

---

### Task 2: 创建缓存 Metrics

**Files:**
- Create: `internal/pkg/cache/metrics.go`
- Create: `internal/pkg/cache/metrics_test.go`

- [ ] **Step 1: Write Metrics implementation**

```go
// internal/pkg/cache/metrics.go
package cache

import (
	"math"
	"sync/atomic"
)

// Metrics collects cache hit/miss statistics.
type Metrics struct {
	hits   atomic.Int64
	misses atomic.Int64
	dbOps  atomic.Int64
	sfs    atomic.Int64
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) Hit()          { m.hits.Add(1) }
func (m *Metrics) Miss()         { m.misses.Add(1) }
func (m *Metrics) DBOp()         { m.dbOps.Add(1) }
func (m *Metrics) SingleFlight() { m.sfs.Add(1) }

func (m *Metrics) HitCount() int64        { return m.hits.Load() }
func (m *Metrics) MissCount() int64       { return m.misses.Load() }
func (m *Metrics) DBOpCount() int64       { return m.dbOps.Load() }
func (m *Metrics) SingleFlightCount() int64 { return m.sfs.Load() }

func (m *Metrics) HitRate() float64 {
	total := m.hits.Load() + m.misses.Load()
	if total == 0 {
		return 0
	}
	rate := float64(m.hits.Load()) / float64(total)
	return math.Round(rate*100) / 100
}
```

- [ ] **Step 2: Write tests**

```go
// internal/pkg/cache/metrics_test.go
package cache

import (
	"math"
	"testing"
)

func TestMetricsHitRate(t *testing.T) {
	m := NewMetrics()
	m.Hit()
	m.Hit()
	m.Miss()
	rate := m.HitRate()
	expected := math.Round((2.0/3.0)*100) / 100
	if rate != expected {
		t.Errorf("expected %.4f, got %.4f", expected, rate)
	}
}

func TestMetricsZeroHits(t *testing.T) {
	m := NewMetrics()
	m.Miss()
	if m.HitRate() != 0 {
		t.Errorf("expected 0, got %.4f", m.HitRate())
	}
}

func TestMetricsCounters(t *testing.T) {
	m := NewMetrics()
	for i := 0; i < 10; i++ {
		m.Hit()
	}
	for i := 0; i < 5; i++ {
		m.Miss()
	}
	for i := 0; i < 3; i++ {
		m.DBOp()
	}
	for i := 0; i < 2; i++ {
		m.SingleFlight()
	}

	if m.HitCount() != 10 {
		t.Errorf("HitCount: want 10, got %d", m.HitCount())
	}
	if m.MissCount() != 5 {
		t.Errorf("MissCount: want 5, got %d", m.MissCount())
	}
	if m.DBOpCount() != 3 {
		t.Errorf("DBOpCount: want 3, got %d", m.DBOpCount())
	}
	if m.SingleFlightCount() != 2 {
		t.Errorf("SingleFlightCount: want 2, got %d", m.SingleFlightCount())
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/pkg/cache/ -run TestMetrics -v
```

Expected: all 3 tests PASS

- [ ] **Step 4: Commit**

```bash
git add internal/pkg/cache/metrics.go internal/pkg/cache/metrics_test.go
git commit -m "feat: add cache metrics collector with hit rate tracking"
```

---

### Task 3: 增强 Redis 缓存封装（Take/Set/Del）

**Files:**
- Modify: `internal/pkg/cache/cache.go` (full rewrite)
- Create: `internal/pkg/cache/cache_test.go`

- [ ] **Step 1: Write tests for Take/Set/Del**

```go
// internal/pkg/cache/cache_test.go
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func newTestRedis(t *testing.T) (*Redis, *miniredis.Miniredis) {
	t.Helper()
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	r := &Redis{Client: client, metrics: NewMetrics()}
	return r, s
}

type testUser struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func TestTakeCacheHit(t *testing.T) {
	r, s := newTestRedis(t)
	defer s.Close()

	data, _ := json.Marshal(testUser{ID: 1, Name: "test"})
	s.Set("user:id:1", string(data))

	var user testUser
	callCount := 0
	err := r.Take(context.Background(), "user:id:1", &user, 1*time.Hour, func() error {
		callCount++
		return errors.New("should not be called")
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != 1 || user.Name != "test" {
		t.Errorf("unexpected user: %+v", user)
	}
	if callCount != 0 {
		t.Error("query should not be called on cache hit")
	}
	if r.metrics.HitCount() != 1 {
		t.Errorf("HitCount: want 1, got %d", r.metrics.HitCount())
	}
}

func TestTakeCacheMiss(t *testing.T) {
	r, s := newTestRedis(t)
	defer s.Close()

	var user testUser
	err := r.Take(context.Background(), "user:id:2", &user, 1*time.Hour, func() error {
		user = testUser{ID: 2, Name: "fetched"}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.Name != "fetched" {
		t.Errorf("unexpected user: %+v", user)
	}
	val, _ := s.Get("user:id:2")
	if val == "" {
		t.Error("data should be cached")
	}
	if r.metrics.MissCount() != 1 {
		t.Errorf("MissCount: want 1, got %d", r.metrics.MissCount())
	}
	if r.metrics.DBOpCount() != 1 {
		t.Errorf("DBOpCount: want 1, got %d", r.metrics.DBOpCount())
	}
}

func TestTakeNotFound(t *testing.T) {
	r, s := newTestRedis(t)
	defer s.Close()

	var user testUser
	err := r.Take(context.Background(), "user:id:999", &user, 1*time.Hour, func() error {
		return ErrNotFound
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	val, _ := s.Get("user:id:999")
	if val != "*" {
		t.Errorf("expected placeholder '*', got %q", val)
	}
}

func TestDelKeys(t *testing.T) {
	r, s := newTestRedis(t)
	defer s.Close()

	s.Set("user:id:1", `{"id":1}`)
	s.Set("user:username:test", `{"id":1}`)

	err := r.Del(context.Background(), "user:id:1", "user:username:test")
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Get("user:id:1")
	if err == nil {
		t.Error("key should be deleted")
	}
}

func TestSetCache(t *testing.T) {
	r, s := newTestRedis(t)
	defer s.Close()

	user := testUser{ID: 10, Name: "settest"}
	err := r.Set(context.Background(), "user:id:10", user, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	val, _ := s.Get("user:id:10")
	if val == "" {
		t.Error("key should exist")
	}
}

func TestTakeSingleFlight(t *testing.T) {
	r, s := newTestRedis(t)
	defer s.Close()

	var callCount int
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			var user testUser
			err := r.Take(context.Background(), "user:sf:1", &user, 1*time.Hour, func() error {
				callCount++
				time.Sleep(50 * time.Millisecond)
				user = testUser{ID: 1, Name: "sf"}
				return nil
			})
			if err != nil {
				t.Error(err)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if callCount != 1 {
		t.Errorf("singleflight: expected 1 call, got %d", callCount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/pkg/cache/ -run "TestTake|TestDel|TestSet" -v 2>&1 | head -20
```

Expected: compilation errors (`Take`, `Set`, `Del`, `ErrNotFound` not defined)

- [ ] **Step 3: Rewrite cache.go with Take/Set/Del**

```go
// internal/pkg/cache/cache.go
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"time"

	"github.com/go-kratos/kratos-layout-monolith/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/v8"
	"github.com/google/wire"
	"golang.org/x/sync/singleflight"
)

const notFoundPlaceholder = "*"

// ErrNotFound indicates the requested record was not found in the database.
var ErrNotFound = errors.New("not found")

// ProviderSet is cache providers.
var ProviderSet = wire.NewSet(NewRedis)

// Redis wraps a Redis client with cache operations.
type Redis struct {
	Client  *redis.Client
	metrics *Metrics
	sf      singleflight.Group
}

// NewRedis creates a new Redis client with cache capabilities.
func NewRedis(c *conf.Redis, logger log.Logger) *Redis {
	if c == nil {
		log.NewHelper(logger).Warn("redis config is nil, skipping redis initialization")
		return &Redis{metrics: NewMetrics()}
	}

	network := c.Network
	if network == "" {
		network = "tcp"
	}

	client := redis.NewClient(&redis.Options{
		Network:      network,
		Addr:         c.Addr,
		Password:     c.Password,
		DB:           c.DB,
		ReadTimeout:  c.ReadTimeout,
		WriteTimeout: c.WriteTimeout,
	})

	helper := log.NewHelper(log.With(logger, "component", "redis"))
	if err := client.Ping(context.Background()).Err(); err != nil {
		helper.Warnf("redis ping failed: %v", err)
	} else {
		helper.Infof("redis connected at %s", c.Addr)
	}

	return &Redis{Client: client, metrics: NewMetrics()}
}

// Close closes the Redis client.
func (r *Redis) Close() error {
	if r.Client != nil {
		return r.Client.Close()
	}
	return nil
}

// Metrics returns the cache metrics collector.
func (r *Redis) Metrics() *Metrics { return r.metrics }

// Take checks cache first, if miss calls queryFn to fetch from DB and caches the result.
// Uses singleflight to prevent cache stampede.
func (r *Redis) Take(ctx context.Context, key string, val any, ttl time.Duration, queryFn func() error) error {
	r.metrics.Miss() // pessimistic count; will be corrected on hit path

	// Try cache first
	data, err := r.Client.Get(ctx, key).Bytes()
	if err == nil && len(data) > 0 {
		if string(data) == notFoundPlaceholder {
			r.metrics.Hit()
			return ErrNotFound
		}
		r.metrics.Hit()
		if jerr := json.Unmarshal(data, val); jerr == nil {
			return nil
		}
		// Corrupted cache, delete and fall through to DB
		r.Client.Del(ctx, key)
	}

	// Use singleflight to prevent cache stampede
	type result struct {
		data []byte
		nf   bool
	}

	val2, _, err := r.sf.Do(key, func() (any, error) {
		r.metrics.DBOp()
		r.metrics.SingleFlight()
		qerr := queryFn()
		if errors.Is(qerr, ErrNotFound) {
			r.Client.Set(ctx, key, notFoundPlaceholder, aroundDuration(60*time.Second))
			return result{nf: true}, nil
		}
		if qerr != nil {
			return nil, qerr
		}

		data, _ := json.Marshal(val)
		r.Client.Set(ctx, key, data, aroundDuration(ttl))
		return result{data: data}, nil
	})
	if err != nil {
		return err
	}

	res := val2.(result)
	if res.nf {
		return ErrNotFound
	}
	return json.Unmarshal(res.data, val)
}

// Set writes a value to cache with the given TTL and random jitter.
func (r *Redis) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return r.Client.Set(ctx, key, data, aroundDuration(ttl)).Err()
}

// Del deletes one or more cache keys.
func (r *Redis) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.Client.Del(ctx, keys...).Err()
}

func aroundDuration(d time.Duration) time.Duration {
	jitter := time.Duration(float64(d) * 0.05 * (rand.Float64()*2 - 1))
	return d + jitter
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/pkg/cache/ -run "TestTake|TestDel|TestSet" -v
```

Expected: all 6 tests PASS

- [ ] **Step 5: Add miniredis test dependency**

```bash
go get github.com/alicebob/miniredis/v2
```

- [ ] **Step 6: Commit**

```bash
git add internal/pkg/cache/cache.go internal/pkg/cache/cache_test.go go.mod go.sum
git commit -m "feat: add Take/Set/Del cache operations with singleflight and metrics"
```

---

### Task 4: 创建 SQL DDL 解析器

**Files:**
- Create: `cmd/genmodel/parser.go`
- Create: `cmd/genmodel/parser_test.go`
- Create: `cmd/genmodel/go.mod` (standalone module, can import parent)

- [ ] **Step 1: Create cmd/genmodel/go.mod**

```bash
cd cmd/genmodel
cat > go.mod << 'EOF'
module github.com/go-kratos/kratos-layout-monolith/cmd/genmodel

go 1.25.0

require (
	github.com/go-kratos/kratos/v2 v2.9.2
	google.golang.org/protobuf v1.36.11
)
EOF
```

> Note: We use a separate go.mod for the CLI tool so it can be `go run` independently. It will use `replace` to reference parent modules if needed. Actually, since the generator doesn't import project packages, it can be fully standalone.

- [ ] **Step 2: Write SQL parser tests**

```go
// cmd/genmodel/parser_test.go
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
```

- [ ] **Step 3: Write SQL parser**

```go
// cmd/genmodel/parser.go
package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Column represents a database column.
type Column struct {
	Name      string
	GoName    string
	GoType    string
	SQLType   string
	Size      int
	Nullable  bool
	Default   string
	IsTime    bool
	AutoIncre bool
	IsPK      bool
}

// UniqueIndex represents a unique index on one or more columns.
type UniqueIndex struct {
	Name       string
	ColumnName string // single-column unique indexes only (current scope)
}

// Table represents a parsed database table.
type Table struct {
	Name          string
	GoName        string
	Columns       []Column
	PrimaryKey    Column
	UniqueIndexes []UniqueIndex
}

var mysqlToGoType = map[string]string{
	"bigint":     "int64",
	"int":        "int64",
	"tinyint":    "int32",
	"smallint":   "int32",
	"mediumint":  "int32",
	"varchar":    "string",
	"char":       "string",
	"text":       "string",
	"tinytext":   "string",
	"mediumtext": "string",
	"longtext":   "string",
	"datetime":   "time.Time",
	"timestamp":  "time.Time",
	"date":       "time.Time",
	"decimal":    "string",
	"float":      "float64",
	"double":     "float64",
	"boolean":    "bool",
	"json":       "string",
}

// ParseSQL parses a CREATE TABLE SQL statement and returns a Table struct.
func ParseSQL(sql string) (*Table, error) {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return nil, fmt.Errorf("empty SQL")
	}

	// Extract table name
	re := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + "`?(\\w+)`?")
	match := re.FindStringSubmatch(sql)
	if len(match) < 2 {
		return nil, fmt.Errorf("cannot parse CREATE TABLE")
	}
	tableName := match[1]

	// Extract body between outer parentheses
	bodyStart := strings.Index(sql, "(")
	bodyEnd := strings.LastIndex(sql, ")")
	if bodyStart == -1 || bodyEnd == -1 || bodyEnd <= bodyStart {
		return nil, fmt.Errorf("cannot find table body")
	}
	body := sql[bodyStart+1 : bodyEnd]

	lines := splitLines(body)

	var columns []Column
	var primaryKey Column
	var uniqueIndexes []UniqueIndex

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// PRIMARY KEY (col)
		if pkRe := regexp.MustCompile(`(?i)^PRIMARY\s+KEY\s*\(([^)]+)\)`); pkRe.MatchString(line) {
			pkMatch := pkRe.FindStringSubmatch(line)
			pkColName := strings.TrimSpace(pkMatch[1])
			for _, c := range columns {
				if c.Name == pkColName {
					c.IsPK = true
					primaryKey = c
					break
				}
			}
			continue
		}

		// UNIQUE KEY idx_name (col)
		if ukRe := regexp.MustCompile(`(?i)^UNIQUE\s+KEY\s+`+"`?(\\w+)`?\\s*\\(([^)]+)\\)"); ukRe.MatchString(line) {
			ukMatch := ukRe.FindStringSubmatch(line)
			idxName := ukMatch[1]
			colName := strings.TrimSpace(strings.Trim(ukMatch[2], "`"))
			uniqueIndexes = append(uniqueIndexes, UniqueIndex{Name: idxName, ColumnName: colName})
			continue
		}

		// Regular KEY / INDEX (skip for cache)
		if regexp.MustCompile(`(?i)^(?:UNIQUE\s+)?KEY\s+`).MatchString(line) {
			continue
		}

		// Skip non-column lines (ENGINE, CHARSET, etc.)
		if !regexp.MustCompile(`(?i)^[`+"`"+`a-z]`).MatchString(line) {
			continue
		}

		col, err := parseColumn(line)
		if err != nil {
			return nil, fmt.Errorf("parse column %q: %w", line, err)
		}
		if col.AutoIncre {
			primaryKey = col
			col.IsPK = true
		}
		columns = append(columns, col)
	}

	if primaryKey.Name == "" {
		return nil, fmt.Errorf("no primary key found in table %s", tableName)
	}

	return &Table{
		Name:          tableName,
		GoName:        toGoName(tableName),
		Columns:       columns,
		PrimaryKey:    primaryKey,
		UniqueIndexes: uniqueIndexes,
	}, nil
}

func parseColumn(line string) (Column, error) {
	line = strings.Trim(line, ",")
	line = strings.TrimSpace(line)

	// Extract column name and rest
	nameRe := regexp.MustCompile("^`?(\\w+)`?\\s+(.+)")
	match := nameRe.FindStringSubmatch(line)
	if match == nil {
		return Column{}, fmt.Errorf("cannot parse column")
	}
	colName := match[1]
	rest := match[2]
	upperRest := strings.ToUpper(rest)

	// Determine SQL type
	var sqlType string
	sqlTypes := []string{"bigint", "mediumint", "smallint", "tinyint", "varchar", "tinytext", "mediumtext", "longtext", "timestamp", "datetime", "boolean", "decimal", "double", "float", "text", "char", "int", "json", "date"}
	for _, t := range sqlTypes {
		if strings.HasPrefix(upperRest, strings.ToUpper(t)) {
			sqlType = t
			break
		}
	}
	if sqlType == "" {
		sqlType = "string"
	}

	goType, ok := mysqlToGoType[sqlType]
	if !ok {
		goType = "string"
	}

	// Extract size from type like varchar(64)
	sizeRe := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(sqlType) + `\((\d+)\)`)
	size := 0
	if m := sizeRe.FindStringSubmatch(rest); len(m) > 1 {
		fmt.Sscanf(m[1], "%d", &size)
	}

	nullable := !strings.Contains(upperRest, "NOT NULL")
	isTime := goType == "time.Time"
	autoIncre := strings.Contains(upperRest, "AUTO_INCREMENT")

	return Column{
		Name:      colName,
		GoName:    toCamel(colName),
		GoType:    goType,
		SQLType:   sqlType,
		Size:      size,
		Nullable:  nullable,
		IsTime:    isTime,
		AutoIncre: autoIncre,
	}, nil
}

func splitLines(s string) []string {
	var result []string
	var current strings.Builder
	parenDepth := 0
	inQuote := false
	for _, r := range s {
		switch {
		case r == '\'' && parenDepth > 0:
			inQuote = !inQuote
			current.WriteRune(r)
		case r == '(' && !inQuote:
			parenDepth++
			current.WriteRune(r)
		case r == ')' && !inQuote:
			parenDepth--
			current.WriteRune(r)
		case r == '\n' && parenDepth == 0 && !inQuote:
			result = append(result, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}
	return result
}

func toGoName(tableName string) string {
	name := toCamel(tableName)
	// Remove trailing 's' for singular model name (users → User)
	if strings.HasSuffix(name, "s") && len(name) > 1 {
		name = name[:len(name)-1]
	}
	return name
}

func toCamel(s string) string {
	parts := strings.Split(s, "_")
	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			runes := []rune(part)
			result.WriteRune(unicode.ToUpper(runes[0]))
			result.WriteString(string(runes[1:]))
		}
	}
	return result.String()
}

func untitle(s string) string {
	if len(s) == 0 {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}
```

- [ ] **Step 4: Run tests**

```bash
cd cmd/genmodel && go test -run TestParse -v && cd ../..
```

Expected: all 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/genmodel/go.mod cmd/genmodel/parser.go cmd/genmodel/parser_test.go
git commit -m "feat: add SQL DDL parser for CREATE TABLE statements"
```

---

### Task 5: 创建代码生成器（CLI + 代码生成逻辑）

**Files:**
- Create: `cmd/genmodel/generator.go`
- Create: `cmd/genmodel/generator_test.go`
- Create: `cmd/genmodel/main.go`

- [ ] **Step 1: Write the code generator**

```go
// cmd/genmodel/generator.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// modelGoTmpl generates the GORM model struct.
const modelGoTmpl = `// Code generated by genmodel. DO NOT EDIT.

package {{.PkgName}}

{{- if .HasTime}}

import "time"
{{end}}

// {{.Table.GoName}} is the GORM model for the {{.Table.Name}} table.
type {{.Table.GoName}} struct {
{{- range .Table.Columns}}
	{{.GoName}} {{.GoType}} {{.GormTag}}
{{- end}}
}

// TableName returns the table name.
func ({{.Table.GoName}}) TableName() string {
	return "{{.Table.Name}}"
}
`

// cacheRepoTmpl generates the cached repository implementation.
const cacheRepoTmpl = `// Code generated by genmodel. DO NOT EDIT.

package {{.PkgName}}

import (
	"context"
	"fmt"
	"time"

	"{{.BizImport}}"
	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
	"{{.CacheImport}}"
)

const (
	{{.CachePrefixVar}} = "{{.CachePrefix}}"
	{{.CacheTTLVar}}    = {{.CacheTTL}}
	{{.CacheJitterVar}} = {{.CacheJitter}}
)

type {{.RepoTypeName}} struct {
	db    *gorm.DB
	redis *cache.Redis
	log   *log.Helper
}

func new{{.Table.GoName}}Repo(db *gorm.DB, rds *cache.Redis, logger log.Logger) *{{.RepoTypeName}} {
	return &{{.RepoTypeName}}{
		db:    db,
		redis: rds,
		log:   log.NewHelper(logger),
	}
}

func (r *{{.RepoTypeName}}) modelCacheKeys(data *{{.Table.GoName}}) []string {
	keys := []string{
		fmt.Sprintf("%sid:%%v", {{.CachePrefixVar}}, data.{{.Table.PrimaryKey.GoName}}),
	}
{{- range .Table.UniqueIndexes}}
	keys = append(keys, fmt.Sprintf("{{.KeyPrefix}}%%v", data.{{.GoName}}))
{{- end}}
	return keys
}

func toBizUser(u *{{.Table.GoName}}) *{{.BizTypeName}} {
	return &{{.BizTypeName}}{
{{- range .BizFromFields}}
		{{.BizField}}: u.{{.GoField}},
{{- end}}
	}
}

func fromBizUser(u *{{.BizTypeName}}) *{{.Table.GoName}} {
	return &{{.Table.GoName}}{
{{- range .BizToFields}}
		{{.GoField}}: u.{{.BizField}},
{{- end}}
	}
}

func (r *{{.RepoTypeName}}) {{.CreateMethodName}}(ctx context.Context, u *{{.BizTypeName}}) (*{{.BizTypeName}}, error) {
	user := fromBizUser(u)
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, err
	}
	r.redis.Set(ctx, fmt.Sprintf("%sid:%%v", {{.CachePrefixVar}}, user.{{.Table.PrimaryKey.GoName}}), user, {{.CacheTTLVar}})
{{- range .Table.UniqueIndexes}}
	r.redis.Set(ctx, fmt.Sprintf("{{.KeyPrefix}}%%v", user.{{.GoName}}), user, {{.CacheTTLVar}})
{{- end}}
	return toBizUser(user), nil
}

func (r *{{.RepoTypeName}}) {{.UpdateMethodName}}(ctx context.Context, u *{{.BizTypeName}}) (*{{.BizTypeName}}, error) {
	// read-before-write: get old record to invalidate old cache keys
	old := &{{.Table.GoName}}{}
	if err := r.db.WithContext(ctx).First(old, u.{{.Table.PrimaryKey.GoName}}).Error; err != nil {
		return nil, err
	}
	oldKeys := r.modelCacheKeys(old)

	if err := r.db.WithContext(ctx).Model(&{{.Table.GoName}}{ID: u.{{.Table.PrimaryKey.GoName}}}).Updates(map[string]any{
{{- range .UpdateColumns}}
		"{{.Name}}": u.{{.GoName}},
{{- end}}
	}).Error; err != nil {
		return nil, err
	}
	r.redis.Del(ctx, oldKeys...)

	// fetch updated record and re-cache
	new := &{{.Table.GoName}}{}
	if err := r.db.WithContext(ctx).First(new, u.{{.Table.PrimaryKey.GoName}}).Error; err != nil {
		return nil, err
	}
	for _, key := range r.modelCacheKeys(new) {
		r.redis.Set(ctx, key, new, {{.CacheTTLVar}})
	}
	return toBizUser(new), nil
}

func (r *{{.RepoTypeName}}) {{.GetByIDMethodName}}(ctx context.Context, id {{.Table.PrimaryKey.GoType}}) (*{{.BizTypeName}}, error) {
	var user {{.Table.GoName}}
	err := r.redis.Take(ctx, fmt.Sprintf("%sid:%%v", {{.CachePrefixVar}}, id), &user, {{.CacheTTLVar}}, func() error {
		return r.db.WithContext(ctx).First(&user, id).Error
	})
	if err != nil {
		return nil, err
	}
	return toBizUser(&user), nil
}
{{- range .Table.UniqueIndexes}}

func (r *{{$.RepoTypeName}}) {{$.GetByMethodName}}(ctx context.Context, {{untitle .GoName}} {{.GoType}}) (*{{$.BizTypeName}}, error) {
	var user {{$.Table.GoName}}
	err := r.redis.Take(ctx, fmt.Sprintf("{{.KeyPrefix}}%%v", {{untitle .GoName}}), &user, {{$.CacheTTLVar}}, func() error {
		return r.db.WithContext(ctx).Where("{{.ColumnName}} = ?", {{untitle .GoName}}).First(&user).Error
	})
	if err != nil {
		return nil, err
	}
	return toBizUser(&user), nil
}
{{- end}}

func (r *{{.RepoTypeName}}) {{.ListMethodName}}(ctx context.Context, page, pageSize int32) ([]*{{.BizTypeName}}, int32, error) {
	var users []{{.Table.GoName}}
	var total int64

	if err := r.db.WithContext(ctx).Model(&{{.Table.GoName}}{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := r.db.WithContext(ctx).
		Offset(int(offset)).
		Limit(int(pageSize)).
		Order("id DESC").
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*{{.BizTypeName}}, 0, len(users))
	for _, u := range users {
		result = append(result, toBizUser(&u))
	}
	return result, int32(total), nil
}

func (r *{{.RepoTypeName}}) {{.DeleteMethodName}}(ctx context.Context, id {{.Table.PrimaryKey.GoType}}) error {
	// read-before-delete: get old record to invalidate cache keys
	old := &{{.Table.GoName}}{}
	if err := r.db.WithContext(ctx).First(old, id).Error; err != nil {
		return err
	}
	r.redis.Del(ctx, r.modelCacheKeys(old)...)
	return r.db.WithContext(ctx).Delete(&{{.Table.GoName}}{}, id).Error
}
`

// GenData holds template data for code generation.
type GenData struct {
	PkgName        string
	ModulePath     string
	BizImport      string
	CacheImport    string
	BizTypeName    string
	Table          *Table
	HasTime        bool
	CachePrefix    string
	CachePrefixVar string
	CacheTTL       string
	CacheTTLVar    string
	CacheJitter    string
	CacheJitterVar string
	RepoTypeName   string
	CreateMethodName    string
	UpdateMethodName    string
	GetByIDMethodName   string
	GetByMethodName     string
	ListMethodName      string
	DeleteMethodName    string
	UpdateColumns  []Column
	BizFromFields  []BizField
	BizToFields    []BizField
}

type BizField struct {
	BizField string
	GoField  string
}

// Generate reads a parsed Table and writes generated Go files to outDir.
func Generate(table *Table, pkgName, modulePath, bizImport, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	data := buildGenData(table, pkgName, modulePath, bizImport)

	// Generate model file
	modelCode, err := executeTemplate("model", modelGoTmpl, data)
	if err != nil {
		return fmt.Errorf("model template: %w", err)
	}
	modelPath := filepath.Join(outDir, snakeToFileName(table.Name)+"_model_gen.go")
	if err := os.WriteFile(modelPath, []byte(modelCode), 0644); err != nil {
		return err
	}

	// Generate cache repo file
	repoCode, err := executeTemplate("cache_repo", cacheRepoTmpl, data)
	if err != nil {
		return fmt.Errorf("cache_repo template: %w", err)
	}
	repoPath := filepath.Join(outDir, snakeToFileName(table.Name)+"_cache_gen.go")
	if err := os.WriteFile(repoPath, []byte(repoCode), 0644); err != nil {
		return err
	}

	return nil
}

func buildGenData(table *Table, pkgName, modulePath, bizImport string) GenData {
	repoType := untitle(table.GoName) + "Repo"
	cacheVarPrefix := untitle(table.GoName) + "CachePrefix"
	cacheTTLVar := untitle(table.GoName) + "CacheTTL"
	cacheJitterVar := untitle(table.GoName) + "CacheJitter"

	hasTime := false
	for _, c := range table.Columns {
		if c.IsTime {
			hasTime = true
			break
		}
	}

	// Update columns: exclude PK, auto-inc, and time fields
	var updateCols []Column
	for _, c := range table.Columns {
		if c.IsPK || c.AutoIncre || c.IsTime {
			continue
		}
		updateCols = append(updateCols, c)
	}

	// Biz conversion fields
	var bizFromFields []BizField
	for _, c := range table.Columns {
		if c.Name == "password" {
			continue // skip password in biz model
		}
		bizFromFields = append(bizFromFields, BizField{
			BizField: c.GoName,
			GoField:  c.GoName,
		})
	}

	var bizToFields []BizField
	for _, c := range table.Columns {
		if c.Name == "password" {
			continue
		}
		bizToFields = append(bizToFields, BizField{
			BizField: c.GoName,
			GoField:  c.GoName,
		})
	}

	return GenData{
		PkgName:           pkgName,
		ModulePath:        modulePath,
		BizImport:         bizImport,
		CacheImport:       modulePath + "/internal/pkg/cache",
		BizTypeName:       "biz." + table.GoName,
		Table:             table,
		HasTime:           hasTime,
		CachePrefix:       table.Name + ":",
		CachePrefixVar:    cacheVarPrefix,
		CacheTTL:          "2 * time.Hour",
		CacheTTLVar:       cacheTTLVar,
		CacheJitter:       "30 * time.Minute",
		CacheJitterVar:    cacheJitterVar,
		RepoTypeName:      repoType,
		CreateMethodName:  "Create" + table.GoName,
		UpdateMethodName:  "Update" + table.GoName,
		GetByIDMethodName: "Get" + table.GoName + "ByID",
		GetByMethodName:   "Get" + table.GoName + "By",
		ListMethodName:    "List" + table.GoName + "s",
		DeleteMethodName:  "Delete" + table.GoName,
		UpdateColumns:     updateCols,
		BizFromFields:     bizFromFields,
		BizToFields:       bizToFields,
	}
}

func executeTemplate(name, tmplText string, data any) (string, error) {
	funcMap := template.FuncMap{
		"lower":   strings.ToLower,
		"untitle": untitle,
	}
	tmpl, err := template.New(name).Funcs(funcMap).Parse(tmplText)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func snakeToFileName(name string) string {
	return strings.ToLower(name)
}
```

- [ ] **Step 2: Write generator test**

```go
// cmd/genmodel/generator_test.go
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
	if !strings.Contains(content, "user:") {
		t.Error("repo file should contain cache prefix")
	}
}
```

- [ ] **Step 3: Run generator test**

```bash
cd cmd/genmodel && go test -run TestGenerate -v && cd ../..
```

Expected: test PASS

- [ ] **Step 4: Write CLI main.go**

```go
// cmd/genmodel/main.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	sqlFile := flag.String("sql", "", "Path to SQL DDL file")
	dir := flag.String("dir", "", "Directory containing SQL files")
	out := flag.String("out", "", "Output directory (default: current dir)")
	biz := flag.String("biz", "", "Biz package import path (e.g., github.com/.../moduser/biz)")
	flag.Parse()

	if *sqlFile == "" && *dir == "" {
		fmt.Fprintln(os.Stderr, "Error: --sql or --dir is required")
		flag.Usage()
		os.Exit(1)
	}

	var sqlFiles []string
	if *sqlFile != "" {
		sqlFiles = append(sqlFiles, *sqlFile)
	}
	if *dir != "" {
		entries, err := os.ReadDir(*dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading directory: %v\n", err)
			os.Exit(1)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
				sqlFiles = append(sqlFiles, filepath.Join(*dir, e.Name()))
			}
		}
	}

	outDir := *out
	if outDir == "" {
		outDir = "."
	}

	for _, f := range sqlFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", f, err)
			os.Exit(1)
		}

		// Support multiple CREATE TABLE statements in one file
		stmts := splitStatements(string(content))
		for _, stmt := range stmts {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || !strings.Contains(strings.ToUpper(stmt), "CREATE TABLE") {
				continue
			}

			table, err := ParseSQL(stmt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", f, err)
				os.Exit(1)
			}

			bizImport := *biz
			if bizImport == "" {
				// Derive from output directory
				absOut, _ := filepath.Abs(outDir)
				parentDir := filepath.Dir(absOut)
				if filepath.Base(absOut) == "data" {
					bizImport = guessModulePath() + strings.TrimPrefix(filepath.ToSlash(parentDir)+"/biz", ".")
				}
			}

			pkgName := filepath.Base(outDir)
			modulePath := guessModulePath()

			if err := Generate(table, pkgName, modulePath, bizImport, outDir); err != nil {
				fmt.Fprintf(os.Stderr, "Error generating %s: %v\n", table.Name, err)
				os.Exit(1)
			}

			fmt.Printf("Generated: %s → %s\n", table.Name, outDir)
		}
	}
}

func splitStatements(content string) []string {
	var stmts []string
	var current strings.Builder
	for _, line := range strings.Split(content, "\n") {
		current.WriteString(line)
		current.WriteString("\n")
		if strings.Contains(line, ");") {
			stmts = append(stmts, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		stmts = append(stmts, current.String())
	}
	return stmts
}

func guessModulePath() string {
	// Read go.mod to find module path
	content, err := os.ReadFile("go.mod")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module ")
		}
	}
	return ""
}
```

- [ ] **Step 5: Commit**

```bash
git add cmd/genmodel/generator.go cmd/genmodel/generator_test.go cmd/genmodel/main.go
git commit -m "feat: add code generator CLI with model and cache repo templates"
```

---

### Task 6: 生成 User 模块代码并重构

**Files:**
- Create: `internal/moduser/data/user_model_gen.go` (generated)
- Create: `internal/moduser/data/user_cache_gen.go` (generated)
- Modify: `internal/moduser/data/user.go` (remove struct + repo impl, keep nothing or minimal helpers)
- Modify: `internal/moduser/data/data.go` (update wire provider)
- Modify: `internal/moduser/biz/user.go` (unchanged — verify interface compatibility)

- [ ] **Step 1: Generate user code using the CLI**

```bash
go run ./cmd/genmodel \
  --sql migrations/001_user.sql \
  --out internal/moduser/data \
  --biz github.com/go-kratos/kratos-layout-monolith/internal/moduser/biz
```

> Note: If `migrations/001_user.sql` doesn't exist yet, create it from the current `User` struct definition:

```sql
-- migrations/001_user.sql
CREATE TABLE users (
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- [ ] **Step 2: Verify generated files compile**

```bash
go build ./internal/moduser/data/
```

Expected: compilation errors (interface mismatch — biz.UserRepo uses different method names, and `toBizUser` returns `time.Time`-based fields vs string)

- [ ] **Step 3: Fix biz conversion for time fields**

The generated `toBizUser` function formats `time.Time` as string. Check the generated code and fix if needed. The current `biz.User` uses `string` for `CreatedAt`/`UpdatedAt`, so the generated code needs:

```go
// In the generated toBizUser function, for time fields:
CreatedAt: u.CreatedAt.Format("2006-01-02 15:04:05"),
UpdatedAt: u.UpdatedAt.Format("2006-01-02 15:04:05"),
```

The template generates `u.CreatedAt` directly. Since `biz.User.CreatedAt` is `string` and `data.User.CreatedAt` is `time.Time`, this won't compile.

Fix: Update the `BizField` generation in `generator.go` to handle time fields specially:

In `buildGenData()`, modify `BizFromFields` to include a flag:

```go
type BizField struct {
    BizField    string
    GoField     string
    IsTimeField bool
}
```

And update the template's `toBizUser` to use `.Format()` for time fields:

```go
{{- range .BizFromFields}}
	{{- if .IsTimeField}}
		{{.BizField}}: u.{{.GoField}}.Format("2006-01-02 15:04:05"),
	{{- else}}
		{{.BizField}}: u.{{.GoField}},
	{{- end}}
{{- end}}
```

- [ ] **Step 4: Update generator.go with time field handling**

Update the `BizField` struct and `buildGenData` function:

```go
type BizField struct {
    BizField    string
    GoField     string
    IsTimeField bool
}
```

And in `buildGenData`:

```go
var bizFromFields []BizField
for _, c := range table.Columns {
    if c.Name == "password" {
        continue
    }
    bizFromFields = append(bizFromFields, BizField{
        BizField:    c.GoName,
        GoField:     c.GoName,
        IsTimeField: c.IsTime,
    })
}
```

Also update the `cacheRepoTmpl` template's `toBizUser` section:

```go
func toBizUser(u *{{.Table.GoName}}) *{{.BizTypeName}} {
    return &{{.BizTypeName}}{
{{- range .BizFromFields}}
    {{- if .IsTimeField}}
        {{.BizField}}: u.{{.GoField}}.Format("2006-01-02 15:04:05"),
    {{- else}}
        {{.BizField}}: u.{{.GoField}},
    {{- end}}
{{- end}}
    }
}
```

- [ ] **Step 5: Refactor data/user.go**

Remove the `User` struct, `userRepo` struct, `NewUserRepo`, all CRUD methods, and `toBizUser` function from `internal/moduser/data/user.go`. The file can be deleted entirely since all code is now generated:

```bash
rm internal/moduser/data/user.go
```

- [ ] **Step 6: Update data/data.go wire provider**

```go
// internal/moduser/data/data.go
package data

import (
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser/biz"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/cache"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"gorm.io/gorm"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewUserRepo)

// NewUserRepo is the wire provider for biz.UserRepo.
func NewUserRepo(db *gorm.DB, rds *cache.Redis, logger log.Logger) biz.UserRepo {
    return newUserRepo(db, rds, logger)
}
```

Wait — the generated `NewUserRepo` (or rather `NewUserRepo` renamed) already returns `biz.UserRepo`. The wire provider just needs to import it.

Actually, the generated function is `NewUserRepo` which takes `(*gorm.DB, *cache.Redis, log.Logger)` and returns `*userRepo`. But `biz.UserRepo` is an interface, so we need the function to return the interface type, or wire will handle it.

Let me reconsider. The generated `NewUserRepo` returns `*userRepo` (concrete type). Since `*userRepo` implements `biz.UserRepo`, wire can provide it as `biz.UserRepo`. We just need to make sure the wire provider is set up correctly.

Updated `data/data.go`:

```go
package data

import "github.com/google/wire"

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewUserRepo)
```

**Wire type matching fix:** The generated `NewUserRepo` returns `*userRepo` (concrete), but wire needs `biz.UserRepo` (interface). Wire won't infer interface implementation from return types. Solution:

1. Generated function is `newUserRepo` (unexported, returns `*userRepo`)
2. `data.go` defines `NewUserRepo` (exported, returns `biz.UserRepo`, calls `newUserRepo`)

- [ ] **Step 7: Regenerate wire**

```bash
go generate ./internal/moduser/
```

Or:

```bash
cd internal/moduser && go run github.com/google/wire/cmd/wire && cd ../..
```

- [ ] **Step 8: Build and test**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 9: Run all tests**

```bash
go test ./...
```

Expected: all tests pass

- [ ] **Step 10: Commit**

```bash
git add internal/moduser/data/user_model_gen.go internal/moduser/data/user_cache_gen.go internal/moduser/data/data.go internal/moduser/data/user.go
git commit -m "refactor: replace user data layer with generated cached repo"
```

---

### Task 7: 提交设计文档并清理

- [ ] **Step 1: Commit design doc**

```bash
git add docs/superpowers/specs/2026-05-08-gorm-cache-codegen-design.md
git commit -m "docs: add gorm cache + codegen design spec"
```

- [ ] **Step 2: Commit plan**

```bash
git add docs/superpowers/plans/2026-05-08-gorm-cache-codegen.md
git commit -m "docs: add implementation plan"
```

---

## 文件总览

| 文件 | 动作 | 说明 |
|------|------|------|
| `go.mod` | 修改 | 添加 `singleflight`, `miniredis` 依赖 |
| `internal/pkg/cache/metrics.go` | 新建 | 缓存指标收集器 |
| `internal/pkg/cache/metrics_test.go` | 新建 | 指标测试 |
| `internal/pkg/cache/cache.go` | 重写 | 添加 Take/Set/Del + singleflight |
| `internal/pkg/cache/cache_test.go` | 新建 | 缓存操作测试 |
| `cmd/genmodel/go.mod` | 新建 | 生成器独立模块 |
| `cmd/genmodel/parser.go` | 新建 | SQL DDL 解析器 |
| `cmd/genmodel/parser_test.go` | 新建 | 解析器测试 |
| `cmd/genmodel/generator.go` | 新建 | 代码生成器（内嵌模板） |
| `cmd/genmodel/generator_test.go` | 新建 | 生成器测试 |
| `cmd/genmodel/main.go` | 新建 | CLI 入口 |
| `internal/moduser/data/user_model_gen.go` | 新建 | 生成的 User GORM Model |
| `internal/moduser/data/user_cache_gen.go` | 新建 | 生成的带缓存 User Repo |
| `internal/moduser/data/user.go` | 删除 | 全部逻辑由生成代码替代 |
| `internal/moduser/data/data.go` | 修改 | wire provider 追加 Redis 依赖 |
| `migrations/001_user.sql` | 新建 | User 表 DDL（生成器输入） |
