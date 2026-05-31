package main

import (
	"context"
	"datacollector/config"
	"datacollector/database"
	"datacollector/label"
	"datacollector/logger"
	"datacollector/plc"
	"datacollector/trassir"
	"datacollector/webserver"
	"flag"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/robinson/gos7"
)

type Line struct {
	Name            string
	IP              string
	Rack            int
	Slot            int
	Camera          string
	Printer         string
	Interval        time.Duration
	DisablePLCWrite bool
	hasDB1013       *bool
}

func main() {
	debugMode := flag.Bool("debug", false, "Режим отладки (НЕ сбрасываем флаги в ПЛК)")
	lineName := flag.String("line", "", "Имя линии для отладки (работаем только с этой линией)")
	overrideIP := flag.String("ip", "", "Переопределить IP адрес ПЛК (для отладки)")
	useDebugDB := flag.Bool("debug-db", false, "Использовать тестовую базу данных для записи")
	flag.Parse()

	if err := logger.Init(); err != nil {
		return
	}
	defer logger.Close()

	// Запуск веб-сервера
	webserver.StartServer(*useDebugDB, logger.LogChannel)

	if err := config.LoadConfig(); err != nil {
		logger.Error("Ошибка загрузки конфигурации: %v", err)
		return
	}

	var connString string
	if *useDebugDB {
		logger.Info("ВНИМАНИЕ: Используется ТЕСТОВАЯ база данных!")
		connString = config.GlobalConfig.DbTestConnection
	} else {
		logger.Info("Запуск работы с БОЕВОЙ базой данных!")
		connString = config.GlobalConfig.DbProdConnection
	}

	if err := database.Init(connString); err != nil {
		logger.Error("Не удалось запустить сборщик: %v", err)
		return
	}
	defer database.Close()

	interval := time.Duration(config.GlobalConfig.PlcPollingIntervalMs) * time.Millisecond

	var activeLines []Line

	// Режим отладки (указан флаг -line)
	if *lineName != "" {
		logger.Info("=== РЕЖИМ ОТЛАДКИ ===")
		logger.Info("Линия: %s", *lineName)
		if *overrideIP != "" {
			logger.Info("IP адрес: %s (переопределён)", *overrideIP)
		}
		if *debugMode {
			logger.Info("Сброс флагов в ПЛК: ОТКЛЮЧЁН")
		} else {
			logger.Info("Сброс флагов в ПЛК: ВКЛЮЧЁН")
		}

		// Загружаем конфигурацию линии из БД
		lineConfig, err := database.GetLineConfig(*lineName)
		if err != nil {
			logger.Error("Ошибка загрузки конфигурации линии %s: %v", *lineName, err)
			return
		}
		if lineConfig == nil {
			logger.Error("Линия %s не найдена в БД или не активна", *lineName)
			return
		}

		// Переопределяем IP если указан флаг
		plcIP := strings.TrimSpace(lineConfig.IP)
		if *overrideIP != "" {
			plcIP = *overrideIP
		}

		// Определяем принтер (если в конфиге линии нет, используем дефолтный)
		printerAddr := config.GlobalConfig.DefaultPrinter
		if lineConfig.Printer.Valid && strings.TrimSpace(lineConfig.Printer.String) != "" {
			printerAddr = strings.TrimSpace(lineConfig.Printer.String)
			logger.Info("Принтер из конфигурации линии: %s", printerAddr)
		} else {
			logger.Info("Принтер из config.json (DefaultPrinter): %s", printerAddr)
		}

		// Определяем камеру
		cameraGuid := "YF8Npzk1" // значение по умолчанию
		if lineConfig.Camera.Valid && lineConfig.Camera.String != "" {
			cameraGuid = lineConfig.Camera.String
		}

		activeLines = []Line{
			{
				Name:            strings.TrimSpace(lineConfig.Name),
				IP:              plcIP,
				Rack:            0,
				Slot:            2,
				Camera:          cameraGuid,
				Printer:         printerAddr,
				Interval:        interval,
				DisablePLCWrite: *debugMode, // В режиме отладки не сбрасываем флаги
			},
		}

		logger.Info("Конфигурация линии:")
		logger.Info("  - Имя: %s", activeLines[0].Name)
		logger.Info("  - IP: %s", activeLines[0].IP)
		logger.Info("  - Принтер: %s", activeLines[0].Printer)
		logger.Info("  - Камера: %s", activeLines[0].Camera)
		logger.Info("  - Сброс флагов: %v", !activeLines[0].DisablePLCWrite)

	} else {
		// Боевой режим - загружаем все активные линии из БД
		logger.Info("=== БОЕВОЙ РЕЖИМ ===")

		dbLines, err := database.GetActiveLines()
		if err != nil {
			logger.Error("Ошибка загрузки линий из БД: %v", err)
			return
		}

		if len(dbLines) == 0 {
			logger.Error("Нет активных линий в таблице plc (is_active=1)")
			return
		}

		for _, dbLine := range dbLines {
			// Получаем камеру (если есть)
			cameraGuid := "YF8Npzk1" // значение по умолчанию
			if dbLine.Camera.Valid {
				cameraGuid = dbLine.Camera.String
			}

			// Получаем принтер (если есть)
			printerAddr := config.GlobalConfig.DefaultPrinter
			if dbLine.Printer.Valid && strings.TrimSpace(dbLine.Printer.String) != "" {
				printerAddr = strings.TrimSpace(dbLine.Printer.String)
			}

			activeLines = append(activeLines, Line{
				Name:            strings.TrimSpace(dbLine.Name),
				IP:              strings.TrimSpace(dbLine.IP),
				Rack:            0,
				Slot:            2,
				Camera:          cameraGuid,
				Printer:         printerAddr,
				Interval:        interval,
				DisablePLCWrite: *debugMode,
				hasDB1013:       nil,
			})

			logger.Info("Добавлена линия: %s (IP: %s, принтер: %s, камера: %s)",
				strings.TrimSpace(dbLine.Name),
				strings.TrimSpace(dbLine.IP),
				printerAddr,
				cameraGuid)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	for _, line := range activeLines {
		wg.Add(1)
		go func(l Line) {
			defer wg.Done()
			runLinePolling(ctx, l)
		}(line)
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	<-stopChan
	logger.Info("Получен сигнал остановки. Завершаем потоки...")
	cancel()
	wg.Wait()
	logger.Info("DataCollector успешно остановлен.")
}

func runLinePolling(ctx context.Context, line Line) {
	logger.Info("[%s] Поток опроса запущен для IP: %s (запись в ПЛК: %v)",
		line.Name, line.IP, !line.DisablePLCWrite)

	handler := gos7.NewTCPClientHandler(line.IP, line.Rack, line.Slot)
	handler.Timeout = 2 * time.Second
	handler.IdleTimeout = 10 * time.Second

	client := gos7.NewClient(handler)
	isConnected := false
	failCounter := 0

	ticker := time.NewTicker(line.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			handler.Close()
			database.UpdateLineOnlineStatus(line.Name, false)
			logger.Info("[%s] Поток опроса остановлен.", line.Name)
			return
		case <-ticker.C:
			if !isConnected {
				if err := handler.Connect(); err != nil {
					failCounter++
					if failCounter%10 == 1 {
						logger.Error("[%s] ПЛК недоступен (попытка %d): %v", line.Name, failCounter, err)
						database.UpdateLineOnlineStatus(line.Name, false)
					}
					continue
				}
				logger.Info("[%s] Связь с ПЛК успешно установлена!", line.Name)
				isConnected = true
				failCounter = 0
				database.UpdateLineOnlineStatus(line.Name, true)
			}

			// Livebit отправляем всегда
			plc.SetFlagAt(client, line.Name, plc.Livebit)

			if !pollPartData(client, &line) || !pollBoxData(client, line) {
				logger.Error("[%s] Ошибка обмена данными. Сбрасываем соединение.", line.Name)
				handler.Close()
				isConnected = false
				database.UpdateLineOnlineStatus(line.Name, false)
			}
		}
	}
}

func pollPartData(client gos7.Client, line *Line) bool {
	// Проверяем наличие DB1013 (один раз при первом вызове)
	if line.hasDB1013 == nil {
		exists := checkDBExists(client, 1013)
		line.hasDB1013 = &exists
		if !exists {
			logger.Info("[%s] DB1013 не найдена в ПЛК, опрос деталей отключён", line.Name)
			return true
		}
		logger.Info("[%s] DB1013 найдена, опрос деталей включён", line.Name)
	}

	// Если DB1013 нет - просто возвращаем успех
	if !*line.hasDB1013 {
		return true
	}

	// Читаем данные из DB1013
	partData := make([]byte, 36)
	err := client.AGReadDB(1013, 0, 36, partData)
	if err != nil {
		logger.Error("[%s] Ошибка чтения DB1013: %v", line.Name, err)
		return false
	}

	if plc.GetBitAt(partData, 2, 2) { // PartReady
		counter := plc.GetIntAt(partData, 32)
		partMaterial := plc.GetStringAt(partData, 14)

		partOk := plc.GetBitAt(partData, 0, 0)
		partNOk := plc.GetBitAt(partData, 0, 1)

		if partOk {
			database.SaveGoodPart(line.Name, partMaterial, counter)
		}

		if partNOk {
			logger.Info("[%s] Деталь имеет дефект (NOK). Выделяем MKM байты...", line.Name)

			mkmData := make([]byte, 4)
			copy(mkmData, partData[26:30])

			go trassir.ProcessNokVideoAsync(line.Name, line.Camera, partMaterial, counter, mkmData)
		}

		// Сбрасываем флаг PartReady если разрешено
		if !line.DisablePLCWrite {
			plc.SetFlagAt(client, line.Name, plc.Partready)
		} else {
			logger.Debug("[%s] Режим отладки: сброс PartReady отключён", line.Name)
		}
	}
	return true
}

func pollBoxData(client gos7.Client, line Line) bool {
	boxData := make([]byte, 64)
	err := client.AGReadDB(1012, 0, 64, boxData)
	if err != nil {
		logger.Error("[%s] Ошибка чтения DB1012: %v", line.Name, err)
		return false // DB1012 критичен, без него останавливаем опрос
	}

	if plc.GetBitAt(boxData, 1, 0) { // BoxReady
		material := plc.GetStringAt(boxData, 2)
		amount := plc.GetRealAt(boxData, 22)
		materialDescription := plc.GetWin1251String(boxData, 28, 36)

		labelCode := database.CloseAndProduceBox(line.Name, material, amount)

		if labelCode != "" {
			boxInfo := label.BoxData{
				LabelCode:   labelCode,
				Material:    material,
				Description: materialDescription,
				Amount:      int(amount),
				Line:        line.Name,
				Shift:       "A",
				Date:        time.Now(),
			}

			go func(info label.BoxData) {
				pdfFile, err := label.GenerateLabelPdf(info, "A5")
				if err != nil {
					logger.Error("[%s] Ошибка генерации PDF: %v", line.Name, err)
					return
				}
				_ = label.PrintLabelNetwork(pdfFile, line.Printer, line.Name)
			}(boxInfo)
		}

		// Сбрасываем флаг BoxReady если разрешено
		if !line.DisablePLCWrite {
			plc.SetFlagAt(client, line.Name, plc.Boxready)
		} else {
			logger.Debug("[%s] Режим отладки: сброс BoxReady отключён", line.Name)
		}
	}
	return true
}

// checkDBExists проверяет существование DB в ПЛК
func checkDBExists(client gos7.Client, dbNumber int) bool {
	// Пробуем прочитать 1 байт из DB
	testData := make([]byte, 36)
	err := client.AGReadDB(dbNumber, 0, 36, testData)
	return err == nil
}
