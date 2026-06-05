package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Material представляет структуру материала из БД
type Material struct {
	MaterialID   int
	MaterialCode string
	CustomerCode string
	Destination  string
	HU           string
	Netto        int
	Brutto       int
	QuantityInHU int
	Description  string
}

// GetMaterialID возвращает ID материала по коду
func GetMaterialID(materialCode string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var materialID int
	query := "SELECT MaterialID FROM [dbo].[materials] WHERE MaterialCode = ?"
	err := DB.QueryRowContext(ctx, query, materialCode).Scan(&materialID)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("материал %s не найден", materialCode)
	}
	if err != nil {
		return 0, fmt.Errorf("ошибка поиска материала: %w", err)
	}
	return materialID, nil
}

// GetMaterialInfo возвращает полную информацию о материале
func GetMaterialInfo(materialCode string) (*Material, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var material Material
	query := `
		SELECT 
			MaterialID,
			MaterialCode,
			CustomerCode,
			Destination,
			HU,
			Netto,
			Brutto,
			QuantityInHU,
			Description
		FROM [dbo].[materials]
		WHERE MaterialCode = ?`

	err := DB.QueryRowContext(ctx, query, materialCode).Scan(
		&material.MaterialID,
		&material.MaterialCode,
		&material.CustomerCode,
		&material.Destination,
		&material.HU,
		&material.Netto,
		&material.Brutto,
		&material.QuantityInHU,
		&material.Description,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("материал %s не найден", materialCode)
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка поиска материала: %w", err)
	}

	return &material, nil
}

// GetMaterialByID возвращает информацию о материале по ID
func GetMaterialByID(materialID int) (*Material, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var material Material
	query := `
		SELECT 
			MaterialID,
			MaterialCode,
			CustomerCode,
			Destination,
			HU,
			Netto,
			Brutto,
			QuantityInHU,
			Description
		FROM [dbo].[materials]
		WHERE MaterialID = ?`

	err := DB.QueryRowContext(ctx, query, materialID).Scan(
		&material.MaterialID,
		&material.MaterialCode,
		&material.CustomerCode,
		&material.Destination,
		&material.HU,
		&material.Netto,
		&material.Brutto,
		&material.QuantityInHU,
		&material.Description,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("материал с ID %d не найден", materialID)
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка поиска материала: %w", err)
	}

	return &material, nil
}

// GetAllMaterials возвращает список всех материалов
func GetAllMaterials() ([]Material, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT 
			MaterialID,
			MaterialCode,
			CustomerCode,
			Destination,
			HU,
			Netto,
			Brutto,
			QuantityInHU,
			Description
		FROM [dbo].[materials]
		ORDER BY MaterialCode`

	rows, err := DB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса материалов: %w", err)
	}
	defer rows.Close()

	var materials []Material
	for rows.Next() {
		var m Material
		err := rows.Scan(
			&m.MaterialID,
			&m.MaterialCode,
			&m.CustomerCode,
			&m.Destination,
			&m.HU,
			&m.Netto,
			&m.Brutto,
			&m.QuantityInHU,
			&m.Description,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования материала: %w", err)
		}
		materials = append(materials, m)
	}

	return materials, nil
}

// GetMaterialQuantityInHU возвращает количество деталей в коробке для материала
func GetMaterialQuantityInHU(materialCode string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var quantity int
	query := "SELECT QuantityInHU FROM [dbo].[materials] WHERE MaterialCode = ?"
	err := DB.QueryRowContext(ctx, query, materialCode).Scan(&quantity)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("материал %s не найден", materialCode)
	}
	if err != nil {
		return 0, fmt.Errorf("ошибка получения QuantityInHU: %w", err)
	}
	return quantity, nil
}
