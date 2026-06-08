package database

import (
	"context"
	"database/sql"
	"datacollector/events"
	"datacollector/logger"
	"fmt"
	"time"
)

type LineConfig struct {
	Name            string
	IP              string
	Port            sql.NullInt64
	Printer         sql.NullString
	PrintLabel      bool
	IsOnline        bool
	LastCheck       time.Time
	IsActive        bool
	Camera          sql.NullString
	lastCounter     int
	lastBoxQuantity int
	lastMaterial    string
}

// GetAllLines загружает все линии из таблицы plc
func GetAllLines() ([]LineConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT 
			[name],
			[ip],
			[port],
			[printer],
			[print_label],
			[is_online],
			[last_check],
			[is_active],
			[camera]
		FROM [dbo].[plc]
		ORDER BY [name]`

	rows, err := DB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса линий: %w", err)
	}
	defer rows.Close()

	var lines []LineConfig
	for rows.Next() {
		var line LineConfig
		var lastCheck sql.NullTime

		err := rows.Scan(
			&line.Name,
			&line.IP,
			&line.Port,
			&line.Printer,
			&line.PrintLabel,
			&line.IsOnline,
			&lastCheck,
			&line.IsActive,
			&line.Camera,
		)
		if err != nil {
			logger.Error("Ошибка сканирования строки линии: %v", err)
			continue
		}

		if lastCheck.Valid {
			line.LastCheck = lastCheck.Time
		}

		lines = append(lines, line)
	}

	return lines, nil
}

// GetActiveLines загружает активные линии из таблицы plc
func GetActiveLines() ([]LineConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT 
			[name],
			[ip],
			[port],
			[printer],
			[print_label],
			[is_online],
			[last_check],
			[is_active],
			[camera]
		FROM [dbo].[plc]
		WHERE [is_active] = 1
		ORDER BY [name]`

	rows, err := DB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса линий: %w", err)
	}
	defer rows.Close()

	var lines []LineConfig
	for rows.Next() {
		var line LineConfig
		var lastCheck sql.NullTime

		err := rows.Scan(
			&line.Name,
			&line.IP,
			&line.Port,
			&line.Printer,
			&line.PrintLabel,
			&line.IsOnline,
			&lastCheck,
			&line.IsActive,
			&line.Camera,
		)
		if err != nil {
			logger.Error("Ошибка сканирования строки линии: %v", err)
			continue
		}

		if lastCheck.Valid {
			line.LastCheck = lastCheck.Time
		}

		lines = append(lines, line)
	}

	return lines, nil
}

// UpdateLineOnlineStatus обновляет статус онлайн/оффлайн линии
func UpdateLineOnlineStatus(lineName string, isOnline bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	query := `
		UPDATE [dbo].[plc] 
		SET 
			[is_online] = ?,
			[last_check] = GETDATE()
		WHERE [name] = ?`

	_, err := DB.ExecContext(ctx, query, isOnline, lineName)
	if err != nil {
		logger.Error("[%s] Ошибка обновления статуса в plc: %v", lineName, err)
		return
	}

	status := "OFFLINE"
	if isOnline {
		status = "ONLINE"
	}
	logger.Info("[%s] Статус линии в БД изменен на: %s", lineName, status)
	events.SendLineStatusEvent(lineName, isOnline)
}

// IsLineActive проверяет, активна ли линия в БД
func IsLineActive(lineName string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var isActive bool
	query := "SELECT is_active FROM [dbo].[plc] WHERE [name] = ?"
	err := DB.QueryRowContext(ctx, query, lineName).Scan(&isActive)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("ошибка проверки активности: %w", err)
	}
	return isActive, nil

}

// UpdateLineStats обновляет статистику линии в БД
func UpdateLineStats(lineName string, counter, boxQuantity int, material string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
        UPDATE [dbo].[plc] 
        SET 
            last_counter = ?,
            last_box_quantity = ?,
            last_material = ?,
            last_check = GETDATE()
        WHERE [name] = ?`

	_, err := DB.ExecContext(ctx, query, counter, boxQuantity, material, lineName)
	if err != nil {
		return fmt.Errorf("ошибка обновления статистики линии: %w", err)
	}
	return nil
}

// GetLineConfig возвращает конфигурацию одной линии по имени
func GetLineConfig(lineName string) (*LineConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT 
			[name],
			[ip],
			[port],
			[printer],
			[print_label],
			[is_online],
			[last_check],
			[is_active],
			[camera]
		FROM [dbo].[plc]
		WHERE [name] = ? AND [is_active] = 1`

	var line LineConfig
	var lastCheck sql.NullTime

	err := DB.QueryRowContext(ctx, query, lineName).Scan(
		&line.Name,
		&line.IP,
		&line.Port,
		&line.Printer,
		&line.PrintLabel,
		&line.IsOnline,
		&lastCheck,
		&line.IsActive,
		&line.Camera,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка поиска линии: %w", err)
	}

	if lastCheck.Valid {
		line.LastCheck = lastCheck.Time
	}

	return &line, nil
}

// GetLineOnlineStatus - получить текущий статус линии
func GetLineOnlineStatus(lineName string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var isOnline bool
	query := "SELECT is_online FROM [dbo].[plc] WHERE name = ?"
	err := DB.QueryRowContext(ctx, query, lineName).Scan(&isOnline)
	if err == sql.ErrNoRows {
		return false, fmt.Errorf("линия %s не найдена", lineName)
	}
	if err != nil {
		return false, fmt.Errorf("ошибка получения статуса: %w", err)
	}
	return isOnline, nil
}
