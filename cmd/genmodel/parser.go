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
	GormTag   string // GORM struct tag, e.g. `gorm:"column:id;primaryKey;autoIncrement"`
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
	"int":        "int32",
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
			for i, c := range columns {
				if c.Name == pkColName {
					columns[i].IsPK = true
					// Append primaryKey to existing tag (preserve autoIncrement etc.)
					tag := strings.Trim(c.GormTag, "`")
					tag = strings.TrimPrefix(tag, "gorm:\"")
					tag = strings.TrimSuffix(tag, "\"")
					if tag != "" && !strings.Contains(tag, "primaryKey") {
						tag += ";primaryKey"
					} else if tag == "" {
						tag = "column:" + pkColName + ";primaryKey"
					}
					columns[i].GormTag = fmt.Sprintf("`%s`", "gorm:\""+tag+"\"")
					primaryKey = columns[i]
					break
				}
			}
			continue
		}

		// UNIQUE KEY idx_name (col)
		if ukRe := regexp.MustCompile(`(?i)^UNIQUE\s+KEY\s+` + "`?(\\w+)`?\\s*\\(([^)]+)\\)"); ukRe.MatchString(line) {
			ukMatch := ukRe.FindStringSubmatch(line)
			idxName := ukMatch[1]
			colName := strings.TrimSpace(strings.Trim(ukMatch[2], "`"))
			uniqueIndexes = append(uniqueIndexes, UniqueIndex{Name: idxName, ColumnName: colName})
			continue
		}

		// Regular KEY / INDEX (skip for cache)
		if regexp.MustCompile(`(?i)^(?:UNIQUE\s+)?(?:KEY|INDEX)\s+`).MatchString(line) {
			continue
		}

		// Skip non-column lines (ENGINE, CHARSET, etc.)
		if !regexp.MustCompile(`(?i)^[` + "`" + `a-z]`).MatchString(line) {
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

	// Determine SQL type — check longest types first to avoid prefix collisions
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

	// Build GORM tag
	var tags []string
	tags = append(tags, "column:"+colName)
	if autoIncre {
		tags = append(tags, "primaryKey")
		tags = append(tags, "autoIncrement")
	}
	if size > 0 {
		tags = append(tags, fmt.Sprintf("size:%d", size))
	}
	gormTag := fmt.Sprintf("`gorm:\"%s\"`", strings.Join(tags, ";"))

	return Column{
		Name:      colName,
		GoName:    toCamel(colName),
		GoType:    goType,
		SQLType:   sqlType,
		Size:      size,
		Nullable:  nullable,
		IsTime:    isTime,
		AutoIncre: autoIncre,
		GormTag:   gormTag,
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
