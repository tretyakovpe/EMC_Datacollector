package main

import (
	"context"
	"datacollector/database" // Импортируем наш пакет БД
	"datacollector/label"
	"datacollector/logger"
	"datacollector/plc"
	"datacollector/trassir"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/robinson/gos7"
)

type Line struct {
	Name     string
	IP       string
	Rack     int
	Slot     int
	Camera   string
	Printer  string
	Interval time.Duration
}

func main() {
	// 1. Объявляем флаги
	debugLineName := flag.String("debug-line", "", "Имя линии для запуска в режиме отладки")
	debugLineIP := flag.String("debug-ip", "", "IP-адрес тестового ПЛК для режима отладки")
	// Добавляем флаг переключения на тестовую базу данных
	useDebugDB := flag.Bool("debug-db", false, "Использовать тестовую базу данных для записи")

	flag.Parse()

	if err := logger.Init(); err != nil {
		return
	}
	defer logger.Close()

	// 2. Настраиваем строки подключения для разных сред
	var connString string

	if *useDebugDB {
		// СТРОКА ПОДКЛЮЧЕНИЯ К ТЕСТОВОЙ БАЗЕ (Копии боевой)
		logger.Info("ВНИМАНИЕ: Используется ТЕСТОВАЯ база данных (emc_prod_dev)!")
		connString = "server=togprod.emc-tlt.tech;user id=sa;password=Mount&Blade-2020;database=emc_prod_dev;encrypt=disable"
	} else {
		// СТРОКА ПОДКЛЮЧЕНИЯ К БОЕВОЙ БАЗЕ
		logger.Info("Запуск работы с БОЕВОЙ базой данных (ProductionDB)!")
		connString = "server=togprod.emc-tlt.tech;user id=sa;password=Mount&Blade-2020;database=emc_prod;encrypt=disable"
	}

	// Инициализация пула соединений с выбранной базой
	if err := database.Init(connString); err != nil {
		logger.Error("Не удалось запустить сборщик: %v", err)
		return
	}
	defer database.Close()

	// Дальнейший код формирования списка активных линий остается без изменений...

	var activeLines []Line

	if *debugLineName != "" {
		testIP := "127.0.0.1"
		if *debugLineIP != "" {
			testIP = *debugLineIP
		}

		activeLines = []Line{
			{
				Name:     *debugLineName,
				IP:       testIP,
				Rack:     0,
				Slot:     2,
				Camera:   "YF8Npzk1",
				Printer:  "togp0004.emc-tlt.tech",
				Interval: 1000 * time.Millisecond,
			},
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
	logger.Info("[%s] Поток опроса запущен для IP: %s", line.Name, line.IP)

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
			// При остановке программы фиксируем, что линия ушла в оффлайн
			database.UpdateLineStatus(line.Name, false)
			logger.Info("[%s] Поток опроса остановлен.", line.Name)
			return
		case <-ticker.C:
			if !isConnected {
				if err := handler.Connect(); err != nil {
					failCounter++
					if failCounter%10 == 1 {
						logger.Error("[%s] ПЛК недоступен (попытка %d): %v", line.Name, failCounter, err)
						// Пишем в БД статус оффлайн
						database.UpdateLineStatus(line.Name, false)
					}
					continue
				}
				logger.Info("[%s] Связь с ПЛК успешно установлена!", line.Name)
				isConnected = true
				failCounter = 0
				// Пишем в БД статус онлайн
				database.UpdateLineStatus(line.Name, true)
			}

			plc.SetFlagAt(client, line.Name, plc.Livebit)

			if !pollPartData(client, line) || !pollBoxData(client, line) {
				logger.Error("[%s] Ошибка обмена данными. Сбрасываем соединение.", line.Name)
				handler.Close()
				isConnected = false
				database.UpdateLineStatus(line.Name, false)
			}
		}
	}
}

func pollPartData(client gos7.Client, line Line) bool {
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
			// ВАЖНО: Фиксируем деталь в базу данных
			database.SaveGoodPart(line.Name, partMaterial, counter)
		}

		if partNOk {
			logger.Info("[%s] Деталь имеет дефект (NOK). Выделяем MKM байты...", line.Name)

			mkmData := make([]byte, 4)
			copy(mkmData, partData[26:30])

			// Передаем line.Camera (Guid) в асинхронный обработчик
			go trassir.ProcessNokVideoAsync(line.Name, line.Camera, partMaterial, counter, mkmData)
		}

		plc.SetFlagAt(client, line.Name, plc.Partready)
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
		materialDescription := plc.GetWin1251String(boxData, 28, 36)

		// 1. Фиксируем в БД и забираем сгенерированный уникальный номер бирки
		labelCode := database.CloseAndProduceBox(line.Name, material, amount)

		if labelCode != "" {
			// 2. Наполняем структуру данными, которые мы только что считали из ПЛК!
			boxInfo := label.BoxData{
				LabelCode:   labelCode,
				Material:    material,
				Description: materialDescription, // Настоящее ASCII имя из станка!
				Amount:      int(amount),
				Line:        line.Name,
				Shift:       "A", // Смена посчитается в базе, для визуализации на бирке поставим пока А
				Date:        time.Now(),
			}

			// 3. Асинхронно пускаем печать, не тормозя конвейер ПЛК
			go func(info label.BoxData) {
				pdfFile, err := label.GenerateLabelPdf(info, "A5")
				if err != nil {
					logger.Error("[%s] Ошибка генерации PDF: %v", line.Name, err)
					return
				}
				// Печать сгенерированной бирки
				_ = label.PrintLabelNetwork(pdfFile, line.Printer, line.Name)
			}(boxInfo)
		}

		plc.SetFlagAt(client, line.Name, plc.Boxready)
	}
	return true
}
