package label

// PrintLabelNetwork отправляет PDF-файл напрямую на сетевой принтер по его IP-адресу (порт 9100)
func PrintLabelNetwork(pdfPath string, printerIP string, lineName string) error {
	/*
		// 1. Читаем готовый PDF-файл с диска
		fileBytes, err := os.ReadFile(pdfPath)
		if err != nil {
			return fmt.Errorf("не удалось прочитать файл бирки перед печатью: %w", err)
		}

		// 2. Указываем стандартный порт прямой печати (9100) для сетевого IP принтера
		printerAddress := fmt.Sprintf("%s:9100", printerIP)

		// 3. Открываем чистое сетевое TCP-соединение с принтером с таймаутом 3 секунды
		conn, err := net.DialTimeout("tcp", printerAddress, 3*time.Second)
		if err != nil {
			// Мягкая обработка: если принтер выключен или это тест, программа НЕ падает
			logger.Error("[%s] [ПЕЧАТЬ] Сетевой принтер %s недоступен. Файл сохранен в pdf/, но печать пропущена.", lineName, printerIP)
			return nil
		}
		defer conn.Close()

		// Устанавливаем таймаут на саму отправку данных (чтобы не зависнуть, если принтер завис)
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

		// 4. Передаем бинарный поток PDF-файла напрямую в сетевую карту принтера
		_, err = conn.Write(fileBytes)
		if err != nil {
			return fmt.Errorf("ошибка при передаче данных в сокет принтера: %w", err)
		}

		logger.Info("[%s] [ПЕЧАТЬ] Бирка успешно отправлена напрямую на сетевой принтер %s.", lineName, printerIP)*/
	return nil
}
