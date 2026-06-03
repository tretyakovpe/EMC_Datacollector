package database

import (
	"context"
	"datacollector/logger"
	"time"
)

// SaveGoodPart фиксирует качественную деталь
// SaveGoodPart фиксирует качественную деталь (обновляет план, НЕ пишет в prod)
func SaveGoodPart(lineName string, materialCode string, counter int) {
	// Получаем MaterialID
	materialID, err := GetMaterialID(materialCode)
	if err != nil {
		logger.Error("[%s] Ошибка получения MaterialID для %s: %v", lineName, materialCode, err)
		return
	}

	// Обновляем ActualAmount в планах
	if err := UpdatePlanActual(materialID, time.Now()); err != nil {
		logger.Error("[%s] Ошибка обновления плана для материала %s: %v", lineName, materialCode, err)
		return
	}

	logger.Info("[%s] Обновлён план для детали %s (№%d)", lineName, materialCode, counter)
}

// SaveBadPart фиксирует бракованную деталь
func SaveBadPart(lineName string, materialCode string, counter int, mkm []byte, videoFileName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `INSERT INTO partNok (name, datetime, counter, mkm, video, line) 
	          VALUES (?, ?, ?, ?, ?, ?)`

	_, err := DB.ExecContext(ctx, query, materialCode, time.Now(), counter, mkm, videoFileName, lineName)
	if err != nil {
		logger.Error("[%s] Ошибка сохранения брака (counter: %d): %v", lineName, counter, err)
		return
	}

	//logger.Info("[%s] Брак детали %s зафиксирован. Видео: %s", lineName, materialCode, videoFileName)
	//events.SendPartEvent(lineName, materialCode, counter, false)
}

// GetTodaysPartsStats возвращает статистику по деталям за сегодня
func GetTodaysPartsStats() (goodParts map[string]int, badParts map[string]int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	today := time.Now().Format("2006-01-02")

	// Годные детали
	goodParts = make(map[string]int)
	queryOK := `SELECT LineName, COUNT(*) FROM prod WHERE CAST(Datetime AS DATE) = ? GROUP BY LineName`
	rowsOK, err := DB.QueryContext(ctx, queryOK, today)
	if err != nil {
		return nil, nil, err
	}
	defer rowsOK.Close()

	for rowsOK.Next() {
		var lineName string
		var count int
		if err := rowsOK.Scan(&lineName, &count); err == nil {
			goodParts[lineName] = count
		}
	}

	// Брак
	badParts = make(map[string]int)
	queryNOK := `SELECT line, COUNT(*) FROM partNok WHERE CAST(datetime AS DATE) = ? GROUP BY line`
	rowsNOK, err := DB.QueryContext(ctx, queryNOK, today)
	if err != nil {
		return goodParts, nil, err
	}
	defer rowsNOK.Close()

	for rowsNOK.Next() {
		var lineName string
		var count int
		if err := rowsNOK.Scan(&lineName, &count); err == nil {
			badParts[lineName] = count
		}
	}

	return goodParts, badParts, nil
}
