package database

import (
	"context"
	"database/sql"
	"datacollector/logger"
	"fmt"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

var DB *sql.DB

// Init инициирует подключение к MS SQL Server
func Init(connectionString string) error {
	var err error
	DB, err = sql.Open("mssql", connectionString)
	if err != nil {
		return fmt.Errorf("ошибка открытия базы данных: %w", err)
	}

	DB.SetMaxOpenConns(50)
	DB.SetMaxIdleConns(10)
	DB.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := DB.PingContext(ctx); err != nil {
		return fmt.Errorf("база данных недоступна: %w", err)
	}

	logger.Info("Успешное подключение к базе данных MS SQL Server.")
	return nil
}

// Close закрывает пул соединений
func Close() {
	if DB != nil {
		DB.Close()
	}
}

// beginTx начинает транзакцию и возвращает tx, cancel функцию и ошибку
func beginTx(timeout time.Duration) (*sql.Tx, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return tx, cancel, nil
}
