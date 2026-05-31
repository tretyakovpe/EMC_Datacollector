package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ShippingOrder представляет отгрузочный ордер
type ShippingOrder struct {
	OrderID     int
	OrderNumber string
	Customer    string
	CreatedAt   time.Time
	ShippedAt   sql.NullTime
}

// PrepareBoxForShipping переводит коробку в статус "Запланирована к отгрузке"
func PrepareBoxForShipping(huNumber string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		UPDATE HU_Status 
		SET Status = N'Запланирована к отгрузке', ChangedAt = GETDATE()
		WHERE HUID IN (
			SELECT TOP 1 HUID FROM HU WHERE HUNumber = ? ORDER BY ChangedAt DESC
		)`

	result, err := DB.ExecContext(ctx, query, huNumber)
	if err != nil {
		return fmt.Errorf("ошибка подготовки к отгрузке: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("коробка %s не найдена", huNumber)
	}

	return nil
}

// ConfirmBoxShipped подтверждает отгрузку коробки
func ConfirmBoxShipped(huNumber string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		UPDATE HU_Status 
		SET Status = N'Отгружена', ChangedAt = GETDATE()
		WHERE HUID IN (
			SELECT TOP 1 HUID FROM HU WHERE HUNumber = ? ORDER BY ChangedAt DESC
		)`

	result, err := DB.ExecContext(ctx, query, huNumber)
	if err != nil {
		return fmt.Errorf("ошибка подтверждения отгрузки: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("коробка %s не найдена", huNumber)
	}

	return nil
}

// GetShippingStatus возвращает статус отгрузки коробки
func GetShippingStatus(huNumber string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var status string
	query := `
		SELECT TOP 1 hs.Status
		FROM HU h
		JOIN HU_Status hs ON h.HUID = hs.HUID
		WHERE h.HUNumber = ?
		ORDER BY hs.ChangedAt DESC`

	err := DB.QueryRowContext(ctx, query, huNumber).Scan(&status)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("коробка %s не найдена", huNumber)
	}
	if err != nil {
		return "", fmt.Errorf("ошибка получения статуса: %w", err)
	}

	return status, nil
}

// GetTodaysShippingStats возвращает статистику отгрузок за сегодня
func GetTodaysShippingStats() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	today := time.Now().Format("2006-01-02")
	query := `
		SELECT COUNT(*)
		FROM HU_Status
		WHERE Status = N'Отгружена'
		AND CAST(ChangedAt AS DATE) = ?`

	var count int
	err := DB.QueryRowContext(ctx, query, today).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("ошибка подсчёта отгрузок: %w", err)
	}
	return count, nil
}
