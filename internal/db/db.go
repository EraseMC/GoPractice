package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/EraseMC/GoPractice/internal/config"
	"github.com/go-sql-driver/mysql"
)

//go:embed migrations/*.sql
var migrations embed.FS

var migrationPattern = regexp.MustCompile(`^V(\d+)__([a-z0-9_]+)\.sql$`)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
	Workers  int
}

func ConfigFromEnv(e config.Env) Config {
	return Config{
		Host:     e.Get("OLDTRALIA_DB_HOST", "127.0.0.1"),
		Port:     e.Int("OLDTRALIA_DB_PORT", 3306),
		Username: e.Get("OLDTRALIA_DB_USERNAME", "oldtralia"),
		Password: e.Get("OLDTRALIA_DB_PASSWORD", ""),
		Database: e.Get("OLDTRALIA_DB_DATABASE", "oldtralia"),
		Workers:  e.Int("OLDTRALIA_DB_WORKERS", 2),
	}
}

func (c Config) dsn() string {
	m := mysql.NewConfig()
	m.User = c.Username
	m.Passwd = c.Password
	m.Net = "tcp"
	m.Addr = net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	m.DBName = c.Database
	m.Collation = "utf8mb4_unicode_ci"
	m.ParseTime = true
	m.Loc = time.Local
	m.Timeout = 10 * time.Second
	return m.FormatDSN()
}

func Open(ctx context.Context, c Config) (*sql.DB, error) {
	d, err := sql.Open("mysql", c.dsn())
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(max(1, c.Workers) + 2)
	d.SetMaxIdleConns(max(1, c.Workers) + 2)
	d.SetConnMaxLifetime(time.Hour)
	d.SetConnMaxIdleTime(10 * time.Minute)
	if err := d.PingContext(ctx); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("could not connect to MySQL %s:%d: %w", c.Host, c.Port, err)
	}
	return d, nil
}

type migration struct {
	version int
	name    string
	path    string
}

func Migrate(ctx context.Context, d *sql.DB, log *slog.Logger) error {
	if _, err := d.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations ("+
		"version INT UNSIGNED NOT NULL PRIMARY KEY, "+
		"name VARCHAR(128) NOT NULL, "+
		"applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP"+
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci"); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied := map[int]bool{}
	rows, err := d.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			_ = rows.Close()
			return err
		}
		applied[v] = true
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return err
	}
	var list []migration
	for _, e := range entries {
		m := migrationPattern.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, _ := strconv.Atoi(m[1])
		list = append(list, migration{version: v, name: m[2], path: "migrations/" + e.Name()})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].version < list[j].version })

	for _, m := range list {
		if applied[m.version] {
			continue
		}
		data, err := migrations.ReadFile(m.path)
		if err != nil {
			return fmt.Errorf("schema %s is missing: %w", m.path, err)
		}
		for _, stmt := range split(string(data)) {
			if _, err := d.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("apply schema V%d %s: %w", m.version, m.name, err)
			}
		}
		if _, err := d.ExecContext(ctx, "INSERT INTO schema_migrations (version, name) VALUES (?, ?)", m.version, m.name); err != nil {
			return fmt.Errorf("record schema V%d: %w", m.version, err)
		}
		log.Info(fmt.Sprintf("Applied schema V%d %s", m.version, m.name))
	}
	return nil
}

func split(sql string) []string {
	var out []string
	for _, s := range strings.Split(sql, ";") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
