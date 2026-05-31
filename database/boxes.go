package database

import (
	"context"
	"database/sql"
	"datacollector/logger"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func CloseAndProduceBox(lineName string, materialCode string, amount float64) string {
	tx, cancel, err := beginTx(5 * time.Second)
	if err != nil {
		logger.Error("[%s] Ошибка старта транзакции: %v", lineName, err)
		return ""
	}
	defer cancel()
	defer tx.Rollback()

	// 1. Получаем ID материала
	materialID, err := GetMaterialID(materialCode)
	if err != nil {
		logger.Error("[%s] %v", lineName, err)
		return ""
	}

	// 2. Ищем запланированную коробку с СОВПАДАЮЩИМ количеством
	var huID int
	var plannedAmount int
	findBoxQuery := `
		SELECT TOP 1 h.HUID, h.Amount
		FROM HU h
		JOIN HU_Status hs ON h.HUID = hs.HUID
		WHERE h.MaterialID = ? 
		  AND h.Amount = ? 
		  AND hs.Status = N'Запланирована'
		ORDER BY hs.ChangedAt ASC`

	err = tx.QueryRow(findBoxQuery, materialID, int(amount)).Scan(&huID, &plannedAmount)
	if err == sql.ErrNoRows {
		logger.Error("[%s] Нет запланированной коробки для материала %s с количеством %d",
			lineName, materialCode, int(amount))
		return ""
	} else if err != nil {
		logger.Error("[%s] Ошибка поиска коробки: %v", lineName, err)
		return ""
	}

	logger.Info("[%s] Найдена запланированная коробка HUID=%d, количество=%d",
		lineName, huID, plannedAmount)

	// 3. Генерируем labelNumber для AddBox (формат: {Line}{YYMMDD}{HHMM})
	now := time.Now()
	lineNameTrimmed := strings.TrimSpace(lineName)

	labelNumber := fmt.Sprintf("%s%s%s",
		lineNameTrimmed,
		now.Format("060102"),
		now.Format("1504"))

	if len(labelNumber) > 12 {
		labelNumber = labelNumber[:12]
	}

	logger.Info("[%s] Сгенерирован labelNumber: %s", lineName, labelNumber)

	// 4. Обновляем HU (только HUNumber, Amount НЕ ТРОГАЕМ!)
	updateHuQuery := "UPDATE HU SET HUNumber = ? WHERE HUID = ?"
	_, err = tx.Exec(updateHuQuery, labelNumber, huID)
	if err != nil {
		logger.Error("[%s] Ошибка обновления HUNumber в HU: %v", lineName, err)
		return ""
	}

	// 5. Добавляем статус "Произведена"
	insertStatusQuery := "INSERT INTO HU_Status (HUID, Status, ChangedAt) VALUES (?, N'Произведена', ?)"
	_, err = tx.Exec(insertStatusQuery, huID, now)
	if err != nil {
		logger.Error("[%s] Ошибка добавления статуса 'Произведена': %v", lineName, err)
		return ""
	}

	// 6. Вызываем процедуру AddBox (она сама запишет в prod и определит смену)
	sqlDate := now.Format("2006-01-02")
	sqlTime := now.Format("15:04:05")

	addBoxProc := `EXEC dbo.AddBox 
		@Date = ?, 
		@Time = ?, 
		@labelNumber = ?, 
		@Name = ?, 
		@Material = ?, 
		@Amount = ?`

	_, err = tx.Exec(addBoxProc, sqlDate, sqlTime, labelNumber, lineName, materialCode, int(amount))
	if err != nil {
		logger.Error("[%s] Ошибка вызова dbo.AddBox: %v", lineName, err)
		return ""
	}

	if err := tx.Commit(); err != nil {
		logger.Error("[%s] Ошибка Commit: %v", lineName, err)
		return ""
	}

	logger.Info("[%s] УСПЕХ: Коробка %s произведена. LabelNumber: %s", lineName, materialCode, labelNumber)
	return labelNumber
}

// GetCurrentBoxInfo возвращает информацию о текущей (последней произведённой) коробке
func GetCurrentBoxInfo(lineName string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		SELECT TOP 1 
			p.label,
			p.material,
			p.amount,
			p.date,
			p.time,
			p.Shift
		FROM prod p
		WHERE p.line = ?
		ORDER BY p.date DESC, p.time DESC`

	var label, material string
	var amount int
	var date string
	var time string
	var shift sql.NullString

	err := DB.QueryRowContext(ctx, query, lineName).Scan(
		&label, &material, &amount, &date, &time, &shift)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка получения последней коробки: %w", err)
	}

	result := map[string]interface{}{
		"label":    label,
		"material": material,
		"amount":   amount,
		"date":     date,
		"time":     time,
		"shift":    nil,
	}

	if shift.Valid {
		result["shift"] = shift.String
	}

	return result, nil
}

// GetTodaysBoxesCount возвращает количество коробок, произведённых сегодня
func GetTodaysBoxesCount() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	today := time.Now().Format("2006-01-02")
	query := `SELECT COUNT(*) FROM prod WHERE date = ?`

	var count int
	err := DB.QueryRowContext(ctx, query, today).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("ошибка подсчёта коробок: %w", err)
	}
	return count, nil
}

// FindLabelPDFPath ищет PDF-файл по номеру бирки в папке pdf/
func FindLabelPDFPath(labelNumber string) (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("не удалось получить путь к программе: %w", err)
	}

	pdfDir := filepath.Join(filepath.Dir(exePath), "pdf")

	pattern := filepath.Join(pdfDir, "*"+labelNumber+"*.pdf")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("ошибка поиска PDF: %w", err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("PDF для бирки %s не найден", labelNumber)
	}

	return matches[0], nil
}
