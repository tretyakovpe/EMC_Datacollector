package database

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"datacollector/config"
	"datacollector/events"
	"datacollector/logger"
)

// UpdatePlanActual увеличивает ActualAmount плана на 1
// Если план не найден, создаёт новый со статусом "Незапланирована" и PlannedAmount = 9999
func UpdatePlanActual(materialID int, t time.Time) error {
	planDate, shift := GetPlanDateAndShift(t)
	planDateStr := planDate.Format("2006-01-02")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Обновляем только ActualAmount, статус НЕ меняем
	query := `
        UPDATE Plans 
        SET ActualAmount = ActualAmount + 1,
            UpdatedAt = GETDATE()
        WHERE MaterialID = ? 
          AND PlanDate = ? 
          AND Shift = ?
    `

	result, err := DB.ExecContext(ctx, query, materialID, planDateStr, shift)
	if err != nil {
		return fmt.Errorf("ошибка обновления плана: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	// Если план не найден, создаём НЕЗАПЛАНИРОВАННЫЙ (статус не меняется)
	if rowsAffected == 0 {
		return CreateUnplannedPlan(materialID, planDateStr, shift)
	}

	return nil
}

func CreateUnplannedPlan(materialID int, planDateStr string, shift string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
        INSERT INTO Plans (PlanDate, Shift, MaterialID, PlannedAmount, ActualAmount, Status, CreatedAt, CreatedBy)
        VALUES (?, ?, ?, 9999, 1, 'Незапланирована', GETDATE(), 'datacollector')
    `

	_, err := DB.ExecContext(ctx, query, planDateStr, shift, materialID)
	if err != nil {
		return fmt.Errorf("ошибка создания незапланированного плана: %w", err)
	}

	logger.Info("[DB] Создан незапланированный план: MaterialID=%d, Date=%s, Shift=%s",
		materialID, planDateStr, shift)

	return nil
}

// sendPlanUpdateEvent отправляет событие об обновлении плана в MES
func SendPlanUpdateEvent(planID, materialID int, planDate time.Time, shift string, plannedAmount, actualAmount int, status string) {
	// Получаем код материала
	materialCode, err := GetMaterialCodeByID(materialID)
	if err != nil {
		logger.Error("[Events] Ошибка получения MaterialCode для ID=%d: %v", materialID, err)
		materialCode = "UNKNOWN"
	}

	eventData := map[string]interface{}{
		"planId":        planID,
		"materialId":    materialID,
		"materialCode":  materialCode,
		"planDate":      planDate.Format("2006-01-02"),
		"shift":         shift,
		"plannedAmount": plannedAmount,
		"actualAmount":  actualAmount,
		"status":        status,
	}

	events.SendEvent("plan_updated", eventData)
}

// GetMaterialCodeByID возвращает код материала по ID
func GetMaterialCodeByID(materialID int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var materialCode string
	query := "SELECT MaterialCode FROM materials WHERE MaterialID = ?"
	err := DB.QueryRowContext(ctx, query, materialID).Scan(&materialCode)
	if err != nil {
		return "", fmt.Errorf("ошибка получения MaterialCode: %w", err)
	}
	return strings.TrimSpace(materialCode), nil
}

// GetPlanDateAndShift определяет производственную дату и смену по времени
// (остаётся без изменений)
func GetPlanDateAndShift(t time.Time) (planDate time.Time, shift string) {
	cfg := config.GlobalConfig
	if cfg.Shifts == nil {
		logger.Error("[GetPlanDateAndShift] Конфигурация смен не загружена")
		return t, "1"
	}

	hour := t.Hour()
	minute := t.Minute()
	currentTime := t

	// Проверяем смены в порядке: 1, 2, 3
	if shiftConfig, ok := cfg.Shifts["1"]; ok {
		start := parseTime(shiftConfig.Start)
		end := parseTime(shiftConfig.End)
		if isTimeInRange(currentTime, start, end, false) {
			return t, "1"
		}
	}

	if shiftConfig, ok := cfg.Shifts["2"]; ok {
		start := parseTime(shiftConfig.Start)
		end := parseTime(shiftConfig.End)
		if isTimeInRange(currentTime, start, end, false) {
			return t, "2"
		}
	}

	if shiftConfig, ok := cfg.Shifts["3"]; ok {
		start := parseTime(shiftConfig.Start)
		end := parseTime(shiftConfig.End)

		if isTimeInRange(currentTime, start, end, true) {
			if hour >= 23 && minute >= 30 {
				return t.AddDate(0, 0, 1), "3"
			}
			return t, "3"
		}
	}

	return t, "1"
}

// parseTime и isTimeInRange остаются без изменений
func parseTime(timeStr string) time.Time {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 3 {
		return time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC)
	}
	hour, _ := strconv.Atoi(parts[0])
	minute, _ := strconv.Atoi(parts[1])
	second, _ := strconv.Atoi(parts[2])
	return time.Date(0, 0, 0, hour, minute, second, 0, time.UTC)
}

func isTimeInRange(t, start, end time.Time, crossesMidnight bool) bool {
	tTime := time.Date(0, 0, 0, t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
	startTime := time.Date(0, 0, 0, start.Hour(), start.Minute(), start.Second(), 0, time.UTC)
	endTime := time.Date(0, 0, 0, end.Hour(), end.Minute(), end.Second(), 0, time.UTC)

	if crossesMidnight {
		return tTime.Compare(startTime) >= 0 || tTime.Compare(endTime) <= 0
	}
	return tTime.Compare(startTime) >= 0 && tTime.Compare(endTime) <= 0
}
