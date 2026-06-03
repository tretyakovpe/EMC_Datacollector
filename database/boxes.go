package database

import (
	"fmt"
	"strings"
	"time"

	"datacollector/logger"
)

// CloseAndProduceBox создаёт коробку со статусом "Произведена"
func CloseAndProduceBox(lineName string, materialCode string, amount float64) string {

	// Генерируем номер бирки
	now := time.Now()
	lineNameTrimmed := strings.TrimSpace(lineName)

	labelNumber := fmt.Sprintf("%s%s%s",
		lineNameTrimmed,
		now.Format("060102"),
		now.Format("1504"))

	if len(labelNumber) > 12 {
		labelNumber = labelNumber[:12]
	}

	logger.Info("[%s] Сгенерирован номер бирки: %s", lineName, labelNumber)

	// Вызываем хранимую процедуру AddBox (она создаст HU, HU_Status и prod)
	sqlDate := now.Format("2006-01-02")
	sqlTime := now.Format("15:04:05")

	addBoxProc := `EXEC dbo.AddBox 
		@Date = ?, 
		@Time = ?, 
		@labelNumber = ?, 
		@Name = ?, 
		@Material = ?, 
		@Amount = ?`
	var err error
	_, err = DB.Exec(addBoxProc, sqlDate, sqlTime, labelNumber, lineName, materialCode, int(amount))
	if err != nil {
		logger.Error("[%s] Ошибка вызова dbo.AddBox: %v", lineName, err)
		return ""
	}

	logger.Info("[%s] Коробка %s произведена. Штрихкод: %s", lineName, materialCode, labelNumber)
	return labelNumber
}
