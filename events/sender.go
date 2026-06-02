package events

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"datacollector/logger"
)

var (
	mesURL = "http://localhost:8080/api/events" // URL вашего MES сервера
	client = &http.Client{Timeout: 5 * time.Second}
)

// SetMESURL устанавливает URL сервера MES
func SetMESURL(url string) {
	mesURL = url
	logger.Info("[Events] MES URL установлен: %s", mesURL)
}

// SendEvent отправляет событие в MES
func SendEvent(eventType string, data interface{}) {
	if mesURL == "" {
		return
	}

	event := map[string]interface{}{
		"type": eventType,
		"data": data,
		"ts":   time.Now().Unix(),
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		logger.Error("[Events] Ошибка маршалинга события: %v", err)
		return
	}

	go func() {
		resp, err := client.Post(mesURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			logger.Debug("[Events] Не удалось отправить событие %s: %v", eventType, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logger.Debug("[Events] MES ответил ошибкой: %d", resp.StatusCode)
		}
	}()
}

// SendBoxClosedEvent отправляет событие о закрытии коробки
func SendBoxClosedEvent(line, material, labelCode string, amount int) {
	SendEvent("box_closed", map[string]interface{}{
		"line":      line,
		"material":  material,
		"labelCode": labelCode,
		"amount":    amount,
	})
}

// SendPartEvent отправляет событие о производстве детали
func SendPartEvent(line, material string, counter int, boxVolume int, isGood bool) {
	eventType := "part_ok"
	if !isGood {
		eventType = "part_nok"
	}
	SendEvent(eventType, map[string]interface{}{
		"line":      line,
		"material":  material,
		"counter":   counter,
		"boxVolume": boxVolume,
	})
}

// SendLineStatusEvent отправляет событие об изменении статуса линии
func SendLineStatusEvent(line string, isOnline bool) {
	SendEvent("line_status", map[string]interface{}{
		"line":     line,
		"isOnline": isOnline,
	})
}

// SendLineActiveEvent отправляет событие об включении/отключении линии
func SendLineActiveEvent(line string, isActive bool) {
	SendEvent("line_active", map[string]interface{}{
		"line":     line,
		"isActive": isActive,
	})
}
