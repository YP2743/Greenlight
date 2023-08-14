package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

const version = "1.0.0"

type config struct {
	port string
	env  string
	db   struct {
		dsn string
	}
}

type application struct {
	config config
	logger *log.Logger
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
	flag.Parse()

	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	db, err := openDB(cfg)
	if err != nil {
		logger.Fatal(err)
	}
	defer db.pool.Close()
	logger.Printf("database connection pool established")

	app := &application{
		config: cfg,
		logger: logger,
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":" + cfg.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	logger.Printf("starting %s server on %s", cfg.env, srv.Addr)
	err = srv.ListenAndServe()
	logger.Fatal(err)
}
