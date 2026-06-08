package main

import (
	"context"
	"datacollector/config"
	"datacollector/database"
	"datacollector/events"
	"datacollector/label"
	"datacollector/logger"
	"datacollector/plc"
	"datacollector/trassir"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kardianos/service"
	"github.com/robinson/gos7"
)

var (
	version   = "1.0.0"
	buildTime = "development"
)

type Line struct {
	Name                 string
	IP                   string
	Rack                 int
	Slot                 int
	Camera               string
	PrintLabel           bool
	Printer              string
	Interval             time.Duration
	DisablePLCWrite      bool
	hasDB1013            *bool
	lastProcessedCounter int
	lastCounter          int
	lastBoxQuantity      int
	lastMaterial         string
}

// program структура для реализации service.Handler
type program struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	lines  []Line
}

// Start запускает программу как службу
func (p *program) Start(s service.Service) error {
	logger.Info("Запуск Data Collector v%s", version)

	// Устанавливаем рабочую директорию в папку с исполняемым файлом
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		if err := os.Chdir(exeDir); err != nil {
			logger.Error("Ошибка смены рабочей директории: %v", err)
		} else {
			logger.Info("Рабочая директория: %s", exeDir)
		}
	}

	// Загружаем конфигурацию
	if err := config.LoadConfig(); err != nil {
		logger.Error("Ошибка загрузки конфигурации: %v", err)
		return err
	}

	// Настраиваем URL для отправки событий в MES
	mesURL := config.GlobalConfig.MESServerURL
	if mesURL == "" {
		mesURL = "http://localhost:8080/api/events"
	}
	events.SetMESURL(mesURL)
	logger.Info("MES сервер: %s", mesURL)

	// Выбираем строку подключения к БД
	connString := config.GlobalConfig.DbProdConnection
	if config.GlobalConfig.DebugMode {
		logger.Info("ВНИМАНИЕ: Используется ТЕСТОВАЯ база данных!")
		connString = config.GlobalConfig.DbTestConnection
	} else {
		logger.Info("Запуск работы с БОЕВОЙ базой данных!")
	}

	// Подключаемся к БД
	if err := database.Init(connString); err != nil {
		logger.Error("Не удалось запустить сборщик: %v", err)
		return err
	}

	interval := time.Duration(config.GlobalConfig.PlcPollingIntervalMs) * time.Millisecond

	// Загружаем линии
	dbLines, err := database.GetAllLines()
	if err != nil {
		logger.Error("Ошибка загрузки линий из БД: %v", err)
		return err
	}

	if len(dbLines) == 0 {
		logger.Error("Нет активных линий в таблице plc (is_active=1)")
		return fmt.Errorf("нет активных линий")
	}

	for _, dbLine := range dbLines {
		cameraGuid := "YF8Npzk1"
		if dbLine.Camera.Valid {
			cameraGuid = dbLine.Camera.String
		}

		printerAddr := config.GlobalConfig.DefaultPrinter
		if dbLine.Printer.Valid && strings.TrimSpace(dbLine.Printer.String) != "" {
			printerAddr = strings.TrimSpace(dbLine.Printer.String)
		}

		p.lines = append(p.lines, Line{
			Name:            strings.TrimSpace(dbLine.Name),
			IP:              strings.TrimSpace(dbLine.IP),
			Rack:            0,
			Slot:            2,
			Camera:          cameraGuid,
			PrintLabel:      dbLine.PrintLabel,
			Printer:         printerAddr,
			Interval:        interval,
			DisablePLCWrite: config.GlobalConfig.DebugMode,
			hasDB1013:       nil,
		})

		logger.Info("Добавлена линия: %s (IP: %s, принтер: %s, камера: %s)",
			strings.TrimSpace(dbLine.Name),
			strings.TrimSpace(dbLine.IP),
			printerAddr,
			cameraGuid)
	}

	// Запускаем опрос линий
	p.ctx, p.cancel = context.WithCancel(context.Background())

	for _, line := range p.lines {
		p.wg.Add(1)
		go func(l Line) {
			defer p.wg.Done()
			runLinePolling(p.ctx, l)
		}(line)
	}

	return nil
}

// Stop останавливает программу
func (p *program) Stop(s service.Service) error {
	logger.Info("Получен сигнал остановки. Завершаем потоки...")
	p.cancel()
	p.wg.Wait()
	database.Close()
	logger.Info("DataCollector успешно остановлен.")
	return nil
}

func main() {
	// Парсим флаги командной строки
	var showVersion bool
	var configPath string
	var installService bool
	var uninstallService bool
	var debugMode bool
	var lineName string
	var useDebugDB bool

	flag.BoolVar(&showVersion, "version", false, "Показать версию")
	flag.StringVar(&configPath, "config", "", "Путь к config.json")
	flag.BoolVar(&installService, "install", false, "Установить как Windows службу")
	flag.BoolVar(&uninstallService, "uninstall", false, "Удалить Windows службу")
	flag.BoolVar(&debugMode, "debug", false, "Режим отладки (НЕ сбрасываем флаги в ПЛК)")
	flag.StringVar(&lineName, "line", "", "Имя линии для отладки (работаем только с этой линией)")
	flag.BoolVar(&useDebugDB, "debug-db", false, "Использовать тестовую базу данных для записи")
	flag.Parse()

	// Инициализируем логгер
	if err := logger.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка инициализации логгера: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	// Устанавливаем флаги в конфиг
	config.GlobalConfig.DebugMode = debugMode
	config.GlobalConfig.DebugDB = useDebugDB
	config.GlobalConfig.DebugLine = lineName

	if showVersion {
		fmt.Printf("Data Collector v%s (built %s)\n", version, buildTime)
		os.Exit(0)
	}

	if configPath != "" {
		config.SetConfigPath(configPath)
	}

	// Настройки для службы
	svcConfig := &service.Config{
		Name:        "EMC_DataCollector",
		DisplayName: "EMC Data Collector",
		Description: "Сбор данных с ПЛК Siemens S7-300",
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		logger.Error("Ошибка создания службы: %v", err)
		os.Exit(1)
	}

	// Обработка установки/удаления службы
	if installService {
		err = s.Install()
		if err != nil {
			logger.Error("Ошибка установки службы: %v", err)
			os.Exit(1)
		}
		logger.Info("Служба EMC_DataCollector успешно установлена")
		logger.Info("Запустите её командой: sc start EMC_DataCollector")
		return
	}

	if uninstallService {
		err = s.Uninstall()
		if err != nil {
			logger.Error("Ошибка удаления службы: %v", err)
			os.Exit(1)
		}
		logger.Info("Служба EMC_DataCollector успешно удалена")
		return
	}

	// Если запущено как служба
	if !service.Interactive() {
		err = s.Run()
		if err != nil {
			logger.Error("Ошибка запуска службы: %v", err)
			os.Exit(1)
		}
		return
	}

	// Интерактивный режим (консоль)
	logger.Info("Запуск в интерактивном режиме")

	// Устанавливаем рабочую директорию в папку с исполняемым файлом
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		if err := os.Chdir(exeDir); err != nil {
			logger.Error("Ошибка смены рабочей директории: %v", err)
		} else {
			logger.Info("Рабочая директория: %s", exeDir)
		}
	}

	// Загружаем конфигурацию
	if err := config.LoadConfig(); err != nil {
		logger.Error("Ошибка загрузки конфигурации: %v", err)
		return
	}

	// Настраиваем URL для отправки событий в MES
	mesURL := config.GlobalConfig.MESServerURL
	if mesURL == "" {
		mesURL = "http://localhost:8080/api/events"
	}
	events.SetMESURL(mesURL)
	logger.Info("MES сервер: %s", mesURL)

	if debugMode {
		logger.Info("Сброс флагов в ПЛК: ОТКЛЮЧЁН")
	} else {
		logger.Info("Сброс флагов в ПЛК: ВКЛЮЧЁН")
	}

	// Выбираем строку подключения к БД
	var connString string
	if useDebugDB {
		logger.Info("ВНИМАНИЕ: Используется ТЕСТОВАЯ база данных!")
		connString = config.GlobalConfig.DbTestConnection
	} else {
		logger.Info("Запуск работы с БОЕВОЙ базой данных!")
		connString = config.GlobalConfig.DbProdConnection
	}

	// Подключаемся к БД
	if err := database.Init(connString); err != nil {
		logger.Error("Не удалось запустить сборщик: %v", err)
		return
	}
	defer database.Close()

	interval := time.Duration(config.GlobalConfig.PlcPollingIntervalMs) * time.Millisecond

	var activeLines []Line

	// Загружаем линии
	if lineName != "" {
		lineConfig, err := database.GetLineConfig(lineName)
		if err != nil {
			logger.Error("Ошибка загрузки конфигурации линии %s: %v", lineName, err)
			return
		}
		if lineConfig == nil {
			logger.Error("Линия %s не найдена в БД или не активна", lineName)
			return
		}

		printerAddr := config.GlobalConfig.DefaultPrinter
		if lineConfig.Printer.Valid && strings.TrimSpace(lineConfig.Printer.String) != "" {
			printerAddr = strings.TrimSpace(lineConfig.Printer.String)
		}

		cameraGuid := "YF8Npzk1"
		if lineConfig.Camera.Valid && lineConfig.Camera.String != "" {
			cameraGuid = lineConfig.Camera.String
		}

		activeLines = []Line{
			{
				Name:            strings.TrimSpace(lineConfig.Name),
				IP:              strings.TrimSpace(lineConfig.IP),
				Rack:            0,
				Slot:            2,
				Camera:          cameraGuid,
				PrintLabel:      true,
				Printer:         printerAddr,
				Interval:        interval,
				DisablePLCWrite: debugMode,
				hasDB1013:       nil,
			},
		}
	} else {
		dbLines, err := database.GetAllLines()
		if err != nil {
			logger.Error("Ошибка загрузки линий из БД: %v", err)
			return
		}
		for _, dbLine := range dbLines {
			cameraGuid := "YF8Npzk1"
			if dbLine.Camera.Valid {
				cameraGuid = dbLine.Camera.String
			}
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
				PrintLabel:      dbLine.PrintLabel,
				Printer:         printerAddr,
				Interval:        interval,
				DisablePLCWrite: debugMode,
				hasDB1013:       nil,
			})
		}
	}

	// Запускаем опрос линий
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

	// Ожидаем сигнал завершения
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)
	<-stopChan

	logger.Info("Получен сигнал остановки. Завершаем потоки...")
	cancel()
	wg.Wait()
	logger.Info("DataCollector успешно остановлен.")
}

// runLinePolling (остаётся без изменений)
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
			isActive, err := database.IsLineActive(line.Name)
			if err != nil {
				logger.Error("[%s] Ошибка проверки активности: %v", line.Name, err)
				continue
			}
			if !isActive {
				//logger.Debug("[%s] Линия неактивна, пропускаем опрос", line.Name)
				continue
			}
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

// Опрос блока DB1013 о каждой детали
func pollPartData(client gos7.Client, line *Line) bool {
	// Проверяем наличие DB1013 (один раз при первом вызове)
	if line.hasDB1013 == nil {
		exists := checkDBExists(client, 1013)
		line.hasDB1013 = &exists
		if !exists {
			logger.Info("[%s] DB1013 не найдена в ПЛК, опрос деталей отключён", line.Name)
			return true
		}
		logger.Debug("[%s] DB1013 найдена, опрос деталей включён", line.Name)
	}

	// Если DB1013 нет - просто возвращаем успех
	if !*line.hasDB1013 {
		return true
	}

	partData := make([]byte, 36)
	err := client.AGReadDB(1013, 0, 36, partData)
	if err != nil {
		logger.Error("[%s] Ошибка чтения DB1013: %v", line.Name, err)
		return false
	}
	counter := plc.GetIntAt(partData, 32)
	partMaterial := plc.GetStringAt(partData, 14)
	boxVolume := plc.GetIntAt(partData, 34)

	// Обновляем статистику в БД, если значения изменились
	if counter != line.lastCounter ||
		boxVolume != line.lastBoxQuantity ||
		partMaterial != line.lastMaterial {

		line.lastCounter = counter
		line.lastBoxQuantity = boxVolume
		line.lastMaterial = partMaterial

		go func(lineName string, c, bq int, mat string) {
			if err := database.UpdateLineStats(lineName, c, bq, mat); err != nil {
				logger.Error("[%s] Ошибка обновления статистики: %v", lineName, err)
			}
		}(line.Name, counter, boxVolume, partMaterial)
	}

	// Отправляем только если counter увеличился
	if counter != line.lastProcessedCounter {
		line.lastProcessedCounter = counter
		// Сообщение для обновления карточек на веб-странице
		events.SendEvent("line_card_update", map[string]interface{}{
			"line":      line.Name,
			"material":  partMaterial,
			"counter":   counter,
			"boxVolume": boxVolume,
		})
		if plc.GetBitAt(partData, 2, 2) { // PartReady
			partOk := plc.GetBitAt(partData, 0, 0)
			partNOk := plc.GetBitAt(partData, 0, 1)
			if partOk {
				database.SaveGoodPart(line.Name, partMaterial, counter)
				logger.Info("[%s] собрано %s %d/%d", line.Name, partMaterial, counter, boxVolume)
				events.SendPartEvent(line.Name, partMaterial, counter, boxVolume, true)
			}

			if partNOk {
				logger.Info("[%s] Деталь имеет дефект (NOK). Выделяем MKM байты...", line.Name)
				events.SendPartEvent(line.Name, partMaterial, counter, boxVolume, true)
				mkmData := make([]byte, 4)
				copy(mkmData, partData[26:30])

				go trassir.ProcessNokVideoAsync(line.Name, line.Camera, partMaterial, counter, mkmData)
			}

			// Сбрасываем флаг PartReady только если НЕ режим отладки
			if !line.DisablePLCWrite {
				plc.SetFlagAt(client, line.Name, plc.Partready)
			}
		}
	}
	return true
}

func pollBoxData(client gos7.Client, line Line) bool {
	boxData := make([]byte, 64)
	err := client.AGReadDB(1012, 0, 64, boxData)
	if err != nil {
		logger.Error("[%s] Ошибка чтения DB1012: %v", line.Name, err)
		return false
	}

	if plc.GetBitAt(boxData, 1, 0) { // BoxReady
		material := plc.GetStringAt(boxData, 2)
		amount := plc.GetRealAt(boxData, 22)
		//materialDescription := plc.GetWin1251String(boxData, 28, 36)
		mat, err := database.GetMaterialInfo(material)
		if err != nil {
			logger.Error("[%s] Ошибка получения материала %s: %v", line.Name, material, err)
			return false
		}
		if mat == nil {
			logger.Error("[%s] Материал %s не найден в БД", line.Name, material)
			return false
		}
		labelCode := database.CloseAndProduceBox(line.Name, material, amount)

		if labelCode != "" && line.PrintLabel {
			boxInfo := label.BoxData{
				LabelCode:      labelCode,
				Material:       mat.MaterialCode,
				CustomerNumber: mat.CustomerCode,
				Destination:    mat.Destination,
				Description:    mat.Description,
				Amount:         int(amount),
				Line:           line.Name,
				Date:           time.Now(),
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

		// Сбрасываем флаг BoxReady только если НЕ режим отладки
		if !line.DisablePLCWrite {
			plc.SetFlagAt(client, line.Name, plc.Boxready)
		}
	}
	return true
}

// checkDBExists проверяет существование DB в ПЛК
func checkDBExists(client gos7.Client, dbNumber int) bool {
	testData := make([]byte, 36)
	err := client.AGReadDB(dbNumber, 0, 36, testData)
	return err == nil
}
