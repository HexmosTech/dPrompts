package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)


func LoadDBConfig(path string) (*DBConfig, error) {
	var conf struct {
		Database DBConfig
	}
	_, err := toml.DecodeFile(path, &conf)
	if err != nil {
		return nil, err
	}
	return &conf.Database, nil
}


func NewDBPool(ctx context.Context, configPath string) (*pgxpool.Pool, error) {

	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal().Err(err).Msg("Unable to determine home directory for config file")
		}
		configPath = homeDir + string(os.PathSeparator) + ".dprompts.toml"
	}
	dbConf, err := LoadDBConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbConf.User, dbConf.Password, dbConf.Host, dbConf.Port, dbConf.Name)

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}

	// Set connection pool settings
	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	// Create the connection pool
	dbPool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Test the connection
	if err := dbPool.Ping(ctx); err != nil {
		dbPool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return dbPool, nil
}
