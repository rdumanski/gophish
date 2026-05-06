package models

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file" // file:// source driver

	gosqlmysql "github.com/go-sql-driver/mysql"
	"github.com/rdumanski/gophish/auth"
	"github.com/rdumanski/gophish/config"

	"github.com/sirupsen/logrus"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite" // registers pure-Go sqlite driver as "sqlite"

	log "github.com/rdumanski/gophish/logger"
)

var db *gorm.DB
var conf *config.Config

const MaxDatabaseConnectionAttempts int = 10

// DefaultAdminUsername is the default username for the administrative user
const DefaultAdminUsername = "admin"

// InitialAdminPassword is the environment variable that specifies which
// password to use for the initial root login instead of generating one
// randomly
const InitialAdminPassword = "GOPHISH_INITIAL_ADMIN_PASSWORD"

// InitialAdminApiToken is the environment variable that specifies the
// API token to seed the initial root login instead of generating one
// randomly
const InitialAdminApiToken = "GOPHISH_INITIAL_ADMIN_API_TOKEN"

const (
	CampaignInProgress string = "In progress"
	CampaignQueued     string = "Queued"
	CampaignCreated    string = "Created"
	CampaignEmailsSent string = "Emails Sent"
	CampaignComplete   string = "Completed"
	EventSent          string = "Email Sent"
	EventSendingError  string = "Error Sending Email"
	EventOpened        string = "Email Opened"
	EventClicked       string = "Clicked Link"
	EventDataSubmit    string = "Submitted Data"
	EventReported      string = "Email Reported"
	EventProxyRequest  string = "Proxied request"
	StatusSuccess      string = "Success"
	StatusQueued       string = "Queued"
	StatusSending      string = "Sending"
	StatusUnknown      string = "Unknown"
	StatusScheduled    string = "Scheduled"
	StatusRetry        string = "Retrying"
	Error              string = "Error"
)

// Flash is used to hold flash information for use in templates.
type Flash struct {
	Type    string
	Message string
}

// Response contains the attributes found in an API response
type Response struct {
	Message string      `json:"message"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

// openGormDialect opens a gorm.DB for the configured driver. The sqlite
// path uses gorm's official driver wired to modernc.org/sqlite (pure Go,
// no CGO). The config-level dbName "sqlite3" is preserved as a backward
// compatibility label for existing config.json files even though the
// underlying database/sql driver is registered as "sqlite".
//
// gorm.io/driver/sqlite still imports github.com/mattn/go-sqlite3 (its
// historical default), but with CGO_ENABLED=0 the mattn init is a no-op
// and only modernc's "sqlite" registration is effective. We pin gorm to
// that driver explicitly via Config.DriverName.
func openGormDialect(dbName, dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	switch dbName {
	case "mysql":
		return gorm.Open(gormmysql.Open(dsn), cfg)
	default:
		return gorm.Open(sqlite.New(sqlite.Config{
			DSN:        dsn,
			DriverName: "sqlite",
		}), cfg)
	}
}

// gormLogrusLogger adapts the package-level logrus.Logger to GORM v2's
// logger.Interface. We default to silent logging (matching the previous
// LogMode(false) behavior) and only emit slow-query / error messages
// through the existing logrus pipeline.
type gormLogrusLogger struct {
	logger        *logrus.Logger
	level         gormlogger.LogLevel
	slowThreshold time.Duration
}

func newGormLogger(l *logrus.Logger) gormlogger.Interface {
	return &gormLogrusLogger{
		logger:        l,
		level:         gormlogger.Silent,
		slowThreshold: 200 * time.Millisecond,
	}
}

func (g *gormLogrusLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	clone := *g
	clone.level = level
	return &clone
}

func (g *gormLogrusLogger) Info(_ context.Context, msg string, args ...interface{}) {
	if g.level >= gormlogger.Info {
		g.logger.Infof(msg, args...)
	}
}

func (g *gormLogrusLogger) Warn(_ context.Context, msg string, args ...interface{}) {
	if g.level >= gormlogger.Warn {
		g.logger.Warnf(msg, args...)
	}
}

func (g *gormLogrusLogger) Error(_ context.Context, msg string, args ...interface{}) {
	if g.level >= gormlogger.Error {
		g.logger.Errorf(msg, args...)
	}
}

func (g *gormLogrusLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	if g.level <= gormlogger.Silent {
		return
	}
	elapsed := time.Since(begin)
	switch {
	case err != nil && g.level >= gormlogger.Error && !errors.Is(err, gorm.ErrRecordNotFound):
		sql, rows := fc()
		g.logger.WithFields(logrus.Fields{
			"elapsed_ms": elapsed.Milliseconds(),
			"rows":       rows,
			"sql":        sql,
		}).Error(err)
	case g.slowThreshold > 0 && elapsed > g.slowThreshold && g.level >= gormlogger.Warn:
		sql, rows := fc()
		g.logger.WithFields(logrus.Fields{
			"elapsed_ms": elapsed.Milliseconds(),
			"rows":       rows,
			"sql":        sql,
		}).Warnf("slow query > %s", g.slowThreshold)
	case g.level >= gormlogger.Info:
		sql, rows := fc()
		g.logger.WithFields(logrus.Fields{
			"elapsed_ms": elapsed.Milliseconds(),
			"rows":       rows,
		}).Debug(sql)
	}
}

// migrationsDir returns the absolute path to the migrations directory for
// the configured database driver.
//
// Two MigrationsPath formats are supported for backward compat:
//   - runtime (config.LoadConfig appends DBName): "db/db_sqlite3"
//     -> we add /migrations
//   - test fixtures (hardcoded full path): "../db/db_sqlite3/migrations/"
//     -> already complete
func migrationsDir(c *config.Config) (string, error) {
	dir := filepath.Clean(c.MigrationsPath)
	if filepath.Base(dir) != "migrations" {
		dir = filepath.Join(dir, "migrations")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve migrations dir: %w", err)
	}
	return abs, nil
}

// newMigrate constructs a *migrate.Migrate bound to the existing *sql.DB.
// We use NewWithDatabaseInstance (not NewWithSourceInstance + URL) so we
// reuse the connection that gorm already owns, avoiding a second sqlite
// open which would conflict with the SetMaxOpenConns(1) setting.
func newMigrate(c *config.Config, sqlDB *sql.DB) (*migrate.Migrate, error) {
	dir, err := migrationsDir(c)
	if err != nil {
		return nil, err
	}
	sourceURL := "file://" + filepath.ToSlash(dir)

	switch c.DBName {
	case "mysql":
		drv, err := migratemysql.WithInstance(sqlDB, &migratemysql.Config{})
		if err != nil {
			return nil, fmt.Errorf("mysql migrate driver: %w", err)
		}
		return migrate.NewWithDatabaseInstance(sourceURL, "mysql", drv)
	default:
		// Pure-Go sqlite via modernc.org/sqlite (the migrate v4 "sqlite"
		// driver). Registers under the name "sqlite", not "sqlite3" — the
		// config-level DBName "sqlite3" is preserved as a backward-compat
		// label for users' existing config.json.
		drv, err := migratesqlite.WithInstance(sqlDB, &migratesqlite.Config{})
		if err != nil {
			return nil, fmt.Errorf("sqlite migrate driver: %w", err)
		}
		return migrate.NewWithDatabaseInstance(sourceURL, "sqlite", drv)
	}
}

// gooseVersionToMigrateBootstrap looks for an existing goose_db_version table
// and, if found, seeds golang-migrate's schema_migrations table with the
// equivalent version. This lets a Gophish 0.12.1 (goose-managed) database
// upgrade in place without re-running migrations that are already applied.
//
// Idempotent: a no-op once schema_migrations is populated.
func gooseVersionToMigrateBootstrap(sqlDB *sql.DB, dbName string) error {
	var hasGoose int
	switch dbName {
	case "mysql":
		err := sqlDB.QueryRow(
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'goose_db_version'",
		).Scan(&hasGoose)
		if err != nil {
			return fmt.Errorf("probe goose_db_version (mysql): %w", err)
		}
	default:
		err := sqlDB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='goose_db_version'",
		).Scan(&hasGoose)
		if err != nil {
			return fmt.Errorf("probe goose_db_version (sqlite3): %w", err)
		}
	}
	if hasGoose == 0 {
		return nil // fresh install or already migrated past goose
	}

	var hasMigrate int
	switch dbName {
	case "mysql":
		err := sqlDB.QueryRow(
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'schema_migrations'",
		).Scan(&hasMigrate)
		if err != nil {
			return fmt.Errorf("probe schema_migrations (mysql): %w", err)
		}
	default:
		err := sqlDB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'",
		).Scan(&hasMigrate)
		if err != nil {
			return fmt.Errorf("probe schema_migrations (sqlite3): %w", err)
		}
	}
	if hasMigrate > 0 {
		return nil // already bootstrapped
	}

	var latestGoose int64
	if err := sqlDB.QueryRow(
		"SELECT version_id FROM goose_db_version WHERE is_applied = 1 ORDER BY id DESC LIMIT 1",
	).Scan(&latestGoose); err != nil {
		return fmt.Errorf("read latest goose version: %w", err)
	}

	// golang-migrate's schema_migrations: (version BIGINT, dirty BOOL).
	// We seed it with the latest goose version, marked clean.
	switch dbName {
	case "mysql":
		_, err := sqlDB.Exec(`CREATE TABLE schema_migrations (
			version BIGINT NOT NULL PRIMARY KEY,
			dirty BOOL NOT NULL
		)`)
		if err != nil {
			return fmt.Errorf("create schema_migrations (mysql): %w", err)
		}
	default:
		_, err := sqlDB.Exec(`CREATE TABLE schema_migrations (
			version uint64 NOT NULL PRIMARY KEY,
			dirty BOOL NOT NULL
		)`)
		if err != nil {
			return fmt.Errorf("create schema_migrations (sqlite3): %w", err)
		}
	}
	if _, err := sqlDB.Exec("INSERT INTO schema_migrations (version, dirty) VALUES (?, ?)", latestGoose, false); err != nil {
		return fmt.Errorf("seed schema_migrations: %w", err)
	}
	log.Infof("Bootstrapped golang-migrate from goose: pinned at version %d", latestGoose)
	return nil
}

// runMigrations applies all pending up migrations. ErrNoChange is treated
// as success.
func runMigrations(c *config.Config, sqlDB *sql.DB) error {
	if err := gooseVersionToMigrateBootstrap(sqlDB, c.DBName); err != nil {
		return err
	}
	m, err := newMigrate(c, sqlDB)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func createTemporaryPassword(u *User) error {
	var temporaryPassword string
	if envPassword := os.Getenv(InitialAdminPassword); envPassword != "" {
		temporaryPassword = envPassword
	} else {
		// This will result in a 16 character password which could be viewed as an
		// inconvenience, but it should be ok for now.
		var err error
		temporaryPassword, err = auth.GenerateSecureKey(auth.MinPasswordLength)
		if err != nil {
			return err
		}
	}
	hash, err := auth.GeneratePasswordHash(temporaryPassword)
	if err != nil {
		return err
	}
	u.Hash = hash
	// Anytime a temporary password is created, we will force the user
	// to change their password
	u.PasswordChangeRequired = true
	err = db.Omit("Role").Save(u).Error
	if err != nil {
		return err
	}
	log.Infof("Please login with the username admin and the password %s", temporaryPassword)
	return nil
}

// Setup initializes the database and runs any needed migrations.
//
// First, it establishes a connection to the database, then runs any migrations
// newer than the version the database is on.
//
// Once the database is up-to-date, we create an admin user (if needed) that
// has a randomly generated API key and password.
func Setup(c *config.Config) error {
	// Setup the package-scoped config
	conf = c

	// Register certificates for tls encrypted db connections
	if conf.DBSSLCaPath != "" {
		switch conf.DBName {
		case "mysql":
			rootCertPool := x509.NewCertPool()
			pem, err := os.ReadFile(conf.DBSSLCaPath)
			if err != nil {
				log.Error(err)
				return err
			}
			if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
				log.Error("Failed to append PEM.")
				return err
			}
			err = gosqlmysql.RegisterTLSConfig("ssl_ca", &tls.Config{
				RootCAs: rootCertPool,
			})
			if err != nil {
				log.Error(err)
				return err
			}
			// Default database is sqlite3, which supports no tls, as connection
			// is file based
		default:
		}
	}

	// Open our database connection
	gormCfg := &gorm.Config{
		Logger: newGormLogger(log.Logger),
	}
	var err error
	i := 0
	for {
		db, err = openGormDialect(conf.DBName, conf.DBPath, gormCfg)
		if err == nil {
			break
		}
		if i >= MaxDatabaseConnectionAttempts {
			log.Error(err)
			return err
		}
		i++
		log.Warn("waiting for database to be up...")
		time.Sleep(5 * time.Second)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Error(err)
		return err
	}
	sqlDB.SetMaxOpenConns(1)

	// Apply pending migrations (golang-migrate). Bootstrapped from any
	// pre-existing goose_db_version table on first run.
	if err := runMigrations(conf, sqlDB); err != nil {
		log.Error(err)
		return err
	}
	// Create the admin user if it doesn't exist
	var userCount int64
	var adminUser User
	db.Model(&User{}).Count(&userCount)
	adminRole, err := GetRoleBySlug(RoleAdmin)
	if err != nil {
		log.Error(err)
		return err
	}
	if userCount == 0 {
		adminUser := User{
			Username:               DefaultAdminUsername,
			Role:                   adminRole,
			RoleID:                 adminRole.ID,
			PasswordChangeRequired: true,
		}

		if envToken := os.Getenv(InitialAdminApiToken); envToken != "" {
			adminUser.ApiKey = envToken
		} else {
			adminUser.ApiKey, err = auth.GenerateSecureKey(auth.APIKeyLength)
			if err != nil {
				log.Error(err)
				return err
			}
		}

		err = db.Omit("Role").Save(&adminUser).Error
		if err != nil {
			log.Error(err)
			return err
		}
	}
	// If this is the first time the user is installing Gophish, then we will
	// generate a temporary password for the admin user.
	//
	// We do this here instead of in the block above where the admin is created
	// since there's the chance the user executes Gophish and has some kind of
	// error, then tries restarting it. If they didn't grab the password out of
	// the logs, then they would have lost it.
	//
	// By doing the temporary password here, we will regenerate that temporary
	// password until the user is able to reset the admin password.
	if adminUser.Username == "" {
		adminUser, err = GetUserByUsername(DefaultAdminUsername)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	if adminUser.PasswordChangeRequired {
		err = createTemporaryPassword(&adminUser)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}
