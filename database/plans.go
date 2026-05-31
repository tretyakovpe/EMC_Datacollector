package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PlannedBox представляет запланированную коробку
type PlannedBox struct {
	HUID         int
	MaterialCode string
	Amount       int
	PlannedDate  time.Time
}

// GetPlannedBoxForMaterial возвращает запланированную коробку для материала
func GetPlannedBoxForMaterial(materialCode string) (*PlannedBox, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		SELECT TOP 1 
			h.HUID,
			m.MaterialCode,
			h.Amount,
			hs.ChangedAt as PlannedDate
		FROM HU h
		JOIN HU_Status hs ON h.HUID = hs.HUID
		JOIN materials m ON h.MaterialID = m.MaterialID
		WHERE m.MaterialCode = ? AND hs.Status = N'Запланирована'
		ORDER BY hs.ChangedAt ASC`

	var box PlannedBox
	var plannedDate time.Time

	err := DB.QueryRowContext(ctx, query, materialCode).Scan(
		&box.HUID, &box.MaterialCode, &box.Amount, &plannedDate)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка поиска плановой коробки: %w", err)
	}

	box.PlannedDate = plannedDate
	return &box, nil
}

// CreateUnplannedBox создаёт коробку вне плана
func CreateUnplannedBox(materialCode string, amount int) (int, error) {
	tx, cancel, err := beginTx(5 * time.Second)
	if err != nil {
		return 0, err
	}
	defer cancel()
	defer tx.Rollback()

	materialID, err := GetMaterialID(materialCode)
	if err != nil {
		return 0, err
	}

	insertQuery := "INSERT INTO HU (MaterialID, Amount) OUTPUT INSERTED.HUID VALUES (?, ?)"
	var huID int
	err = tx.QueryRow(insertQuery, materialID, amount).Scan(&huID)
	if err != nil {
		return 0, fmt.Errorf("ошибка создания HU: %w", err)
	}

	// Добавляем статус "Запланирована"
	statusQuery := "INSERT INTO HU_Status (HUID, Status, ChangedAt) VALUES (?, N'Запланирована', ?)"
	_, err = tx.Exec(statusQuery, huID, time.Now())
	if err != nil {
		return 0, fmt.Errorf("ошибка добавления статуса: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return huID, nil
}

// UpdateBoxStatus обновляет статус коробки
func UpdateBoxStatus(huID int, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := "INSERT INTO HU_Status (HUID, Status, ChangedAt) VALUES (?, ?, ?)"
	_, err := DB.ExecContext(ctx, query, huID, status, time.Now())
	if err != nil {
		return fmt.Errorf("ошибка обновления статуса: %w", err)
	}
	return nil
}
