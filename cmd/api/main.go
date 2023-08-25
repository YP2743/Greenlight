package main

import (
	"context"
	"flag"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"greenlight.yp2743.me/internal/data"
	"greenlight.yp2743.me/internal/jsonlog"
	"greenlight.yp2743.me/internal/mailer"
)

const version = "1.0.0"

type config struct {
	port string
	env  string
	db   struct {
		dsn          string
		maxOpenConns string
		maxIdleTime  string
	}
	limiter struct {
		rps     string
		burst   string
		enabled bool
	}
	smtp struct {
		host     string
		port     string
		username string
		password string
		sender   string
	}
	cors struct {
		trustedOrigins []string
	}
}

type application struct {
	config config
	logger *jsonlog.Logger
	models data.Models
	mailer mailer.Mailer
	wg     sync.WaitGroup
}

// Singleton pattern to make sure that only one connection pool exists.
type postgres struct {
	pool *pgxpool.Pool
}

var (
	pgInstance *postgres
	pgOnce     sync.Once
)

func openDB(cfg config) (*postgres, error) {
	var err error
	pgOnce.Do(func() {
		var db *pgxpool.Pool
		db, err = pgxpool.New(context.Background(), cfg.db.dsn)
		if err != nil {
			return
		}

		i, err := strconv.Atoi(cfg.db.maxOpenConns)
		if err != nil {
			return
		}
		db.Config().MaxConns = int32(i)

		duration, err := time.ParseDuration(cfg.db.maxIdleTime)
		if err != nil {
			return
		}
		db.Config().MaxConnIdleTime = duration

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Check connection within the 5-second deadline.
		err = db.Ping(ctx)
		if err == nil {
			pgInstance = &postgres{pool: db}
		}
	})
	return pgInstance, nil
}

func main() {

	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)

	err := godotenv.Load()
	if err != nil {
		logger.PrintFatal(err, nil)
	}

	var cfg config

	flag.StringVar(&cfg.port, "port", os.Getenv("PORT"), "API server port")
	flag.StringVar(&cfg.env, "env", os.Getenv("ENVIRONMENT"), "Environment (development|staging|production)")

	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("DB_URL"), "PostgreSQL DSN")
	flag.StringVar(&cfg.db.maxOpenConns, "db-max-open-conns", os.Getenv("DB_MAX_OPEN_CONNS"), "PostgreSQL max open connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", os.Getenv("DB_MAX_IDLE_TIME"), "PostgreSQL max connection idle time")

	flag.StringVar(&cfg.limiter.rps, "limiter-rps", os.Getenv("RPS_LIMIT"), "Rate limiter maximum requests per second")
	flag.StringVar(&cfg.limiter.burst, "limiter-burst", os.Getenv("BURST_LIMIT"), "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	flag.StringVar(&cfg.smtp.host, "smtp-host", os.Getenv("SMTP_HOST"), "SMTP host")
	flag.StringVar(&cfg.smtp.port, "smtp-port", os.Getenv("SMTP_PORT"), "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", os.Getenv("SMTP_USERNAME"), "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", os.Getenv("SMTP_PASSWORD"), "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", os.Getenv("SMTP_SENDER"), "SMTP sender")

	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})

	flag.Parse()

	db, err := openDB(cfg)
	if err != nil {
		logger.PrintFatal(err, nil)
	}
	defer db.pool.Close()
	logger.PrintInfo("database connection pool established", nil)

	smtp_port, err := strconv.Atoi(cfg.smtp.port)
	if err != nil {
		return
	}
	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db.pool),
		mailer: mailer.New(cfg.smtp.host, smtp_port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
	}

	err = app.serve()
	if err != nil {
		logger.PrintFatal(err, nil)
	}
}
