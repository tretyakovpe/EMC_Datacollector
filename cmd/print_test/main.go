package main

import (
	"datacollector/config"
	"datacollector/logger"
	"datacollector/trassir"
	"flag"
	"fmt"
	"time"
)

func main() {
	var lineName string
	var cameraGuid string
	var materialCode string
	var counter int

	flag.StringVar(&lineName, "line", "25", "Имя линии")
	flag.StringVar(&cameraGuid, "camera", "YF8Npzk1", "GUID камеры")
	flag.StringVar(&materialCode, "material", "LO2200-100", "Код материала")
	flag.IntVar(&counter, "counter", 1, "Номер детали")
	flag.Parse()

	// Инициализируем логгер
	if err := logger.Init(); err != nil {
		fmt.Printf("Ошибка инициализации логгера: %v\n", err)
		return
	}
	defer logger.Close()

	// Загружаем конфиг
	if err := config.LoadConfig(); err != nil {
		logger.Error("Ошибка загрузки конфига: %v", err)
		return
	}

	logger.Info("=== ТЕСТ TRASSIR ===")
	logger.Info("Линия: %s", lineName)
	logger.Info("Камера: %s", cameraGuid)
	logger.Info("Материал: %s", materialCode)
	logger.Info("Счётчик: %d", counter)
	logger.Info("Время: %s", time.Now().Format("15:04:05"))

	// Тестовые MKM байты
	mkm := []byte{0x01, 0x02, 0x03, 0x04}

	logger.Info("Запуск ProcessNokVideoAsync...")
	trassir.ProcessNokVideoAsync(lineName, cameraGuid, materialCode, counter, mkm)

	logger.Info("Ожидаем завершения (30 сек)...")
	time.Sleep(30 * time.Second)

	logger.Info("=== ТЕСТ ЗАВЕРШЁН ===")
}
