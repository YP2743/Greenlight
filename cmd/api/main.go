package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"greenlight.yp2743.me/internal/data"
	"greenlight.yp2743.me/internal/jsonlog"
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
}

type application struct {
	config config
	logger *jsonlog.Logger
	models data.Models
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

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	var cfg config

	flag.StringVar(&cfg.port, "port", os.Getenv("PORT"), "API server port")
	flag.StringVar(&cfg.env, "env", os.Getenv("ENVIRONMENT"), "Environment (development|staging|production)")
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("DB_URL"), "PostgreSQL DSN")
	flag.StringVar(&cfg.db.maxOpenConns, "db-max-open-conns", os.Getenv("DB_MAX_OPEN_CONNS"), "PostgreSQL max open connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", os.Getenv("DB_MAX_IDLE_TIME"), "PostgreSQL max connection idle time")
	flag.Parse()

	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)

	db, err := openDB(cfg)
	if err != nil {
		logger.PrintFatal(err, nil)
	}
	defer db.pool.Close()
	logger.PrintInfo("database connection pool established", nil)

	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db.pool),
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":" + cfg.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	logger.PrintInfo("starting server", map[string]string{
		"addr": srv.Addr,
		"env":  cfg.env,
	})

	err = srv.ListenAndServe()
	logger.PrintFatal(err, nil)
}
