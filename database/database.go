package database

import (
	"context"
	"database/sql"
	"datacollector/logger"
	"fmt"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

// DB — глобальный пул соединений с базой данных
var DB *sql.DB

// Init инициирует подключение к MS SQL Server
func Init(connectionString string) error {
	var err error
	// Открываем пул соединений
	DB, err = sql.Open("mssql", connectionString)
	if err != nil {
		return fmt.Errorf("ошибка открытия базы данных: %w", err)
	}

	// Настройки пула соединений (важно для параллельного опроса)
	DB.SetMaxOpenConns(50)                  // Максимум 50 одновременных соединений
	DB.SetMaxIdleConns(10)                  // Держать до 10 свободных соединений в памяти
	DB.SetConnMaxLifetime(30 * time.Minute) // Переоткрывать соединения через 30 мин

	// Проверяем физическое наличие связи с сервером БД
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := DB.PingContext(ctx); err != nil {
		return fmt.Errorf("база данных недоступна: %w", err)
	}

	logger.Info("Успешное подключение к базе данных MS SQL Server.")
	return nil
}

// Close закрывает пул соединений при остановке приложения
func Close() {
	if DB != nil {
		DB.Close()
	}
}

// UpdateLineStatus вызывает хранимую процедуру UpdateIsOnline
func UpdateLineStatus(lineName string, isOnline bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Вызываем процедуру с явным указанием именованных параметров SQL Server
	_, err := DB.ExecContext(ctx, "EXEC dbo.UpdateIsOnline @Name = ?, @IsOnline = ?", lineName, isOnline)
	if err != nil {
		logger.Error("[%s] Ошибка вызова процедуры UpdateIsOnline в БД: %v", lineName, err)
		return
	}

	// Не пишем лог каждую секунду, чтобы не забивать файл,
	// но при смене статуса на оффлайн это будет полезно увидеть
	if !isOnline {
		logger.Info("[%s] Статус линии в БД изменен на: Offline", lineName)
	}
}

// SaveGoodPart фиксирует готовую качественную деталь (OK) в базу данных
func SaveGoodPart(lineName string, materialCode string, counter int) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Пример прямого лаконичного SQL-запроса (в лоб) без ORM-магии.
	// Замените имя таблицы и колонок на ваши реальные, если они отличаются.
	query := "INSERT INTO prod (MaterialCode, LineName, Counter, Datetime) VALUES (?, ?, ?, ?)"
	_, err := DB.ExecContext(ctx, query, materialCode, lineName, counter, time.Now())
	if err != nil {
		logger.Error("[%s] Ошибка сохранения детали %s в БД: %v", lineName, materialCode, err)
		return
	}
	logger.Info("[%s] Деталь %s (№%d) успешно записана в БД.", lineName, materialCode, counter)
}

// CloseAndProduceBox переводит коробку в статус "Произведена" и вызывает процедуру AddBox
func CloseAndProduceBox(lineName string, materialCode string, amount float64) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Начинаем транзакцию, чтобы защитить данные от сбоев
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		logger.Error("[%s] Ошибка старта транзакции для закрытия ящика: %v", lineName, err)
		return ""
	}
	defer tx.Rollback()

	// 1. Ищем ID материала
	var materialID int
	err = tx.QueryRowContext(ctx, "SELECT MaterialID FROM materials WHERE MaterialCode = ?", materialCode).Scan(&materialID)
	if err != nil {
		logger.Error("[%s] Материал %s не найден в справочнике: %v", lineName, materialCode, err)
		return ""
	}

	// 2. Ищем запланированную коробку на сегодня
	var huID int
	findBoxQuery := `
		SELECT TOP 1 h.HUID 
		FROM HU h
		JOIN HU_Status hs ON h.HUID = hs.HUID
		WHERE h.MaterialID = ? AND hs.Status = N'Запланирована'
		ORDER BY hs.ChangedAt ASC`

	err = tx.QueryRowContext(ctx, findBoxQuery, materialID).Scan(&huID)
	if err == sql.ErrNoRows {
		// Бэкапный сценарий: если коробки нет в плане, создаем её "вне плана"
		logger.Error("[%s] Предупреждение: Коробка для %s отсутствует в плане! Создаем вне плана.", lineName, materialCode)
		insertHuQuery := "INSERT INTO HU (MaterialID, Amount) OUTPUT INSERTED.HUID VALUES (?, ?)"
		err = tx.QueryRowContext(ctx, insertHuQuery, materialID, int(amount)).Scan(&huID)
		if err != nil {
			logger.Error("[%s] Ошибка создания HU вне плана: %v", lineName, err)
			return ""
		}
	} else if err != nil {
		logger.Error("[%s] Ошибка поиска коробки в БД: %v", lineName, err)
		return ""
	}

	// 3. Генерируем уникальный 12-значный штрихкод бирки (в процедуре NVARCHAR(12))
	// Маска: Год(2)Месяц(2)День(2) + Хвост из ID коробки, дополненный нулями до 12 символов
	now := time.Now()
	labelNumber := fmt.Sprintf("%s%06d", now.Format("060102"), huID)
	if len(labelNumber) > 12 {
		// Защита: если строка превысила 12 символов, берем последние 12
		labelNumber = labelNumber[len(labelNumber)-12:]
	}

	// 4. Обновляем параметры Handling Unit (HU)
	updateHuQuery := "UPDATE HU SET Amount = ?, HUNumber = ? WHERE HUID = ?"
	_, err = tx.ExecContext(ctx, updateHuQuery, int(amount), labelNumber, huID)
	if err != nil {
		logger.Error("[%s] Ошибка обновления параметров HU: %v", lineName, err)
		return ""
	}

	// 5. Добавляем статус "Произведена" в историю
	insertStatusQuery := "INSERT INTO HU_Status (HUID, Status, ChangedAt) VALUES (?, N'Произведена', ?)"
	_, err = tx.ExecContext(ctx, insertStatusQuery, huID, now)
	if err != nil {
		logger.Error("[%s] Ошибка добавления статуса 'Произведена': %v", lineName, err)
		return ""
	}

	// 6. ВЫЗЫВАЕМ ВАШУ БОЕВУЮ ПРОЦЕДУРУ dbo.AddBox
	// Подготавливаем формат даты (YYYY-MM-DD) и времени (HH:MM:SS) для SQL Server
	sqlDate := now.Format("2006-01-02")
	sqlTime := now.Format("15:04:05")

	addBoxProc := `EXEC dbo.AddBox 
		@Date = ?, 
		@Time = ?, 
		@labelNumber = ?, 
		@Name = ?, 
		@Material = ?, 
		@Amount = ?`

	_, err = tx.ExecContext(ctx, addBoxProc, sqlDate, sqlTime, labelNumber, lineName, materialCode, int(amount))
	if err != nil {
		logger.Error("[%s] Ошибка вызова процедуры dbo.AddBox: %v", lineName, err)
		return ""
	}

	// Фиксируем транзакцию (Всё или ничего)
	if err := tx.Commit(); err != nil {
		logger.Error("[%s] Ошибка Commit транзакции: %v", lineName, err)
		return ""
	}

	logger.Info("[%s] УСПЕХ: Ящик %s успешно зафиксирован! Процедура AddBox выполнена. Штрихкод бирки: %s",
		lineName, materialCode, labelNumber)
	return labelNumber
}

// SaveBadPart фиксирует бракованную деталь (NOK) и имя файла видео в таблицу partNok
func SaveBadPart(lineName string, materialCode string, counter int, mkm []byte, videoFileName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Названия колонок берем строго из структуры вашей модели C# (в нижнем регистре)
	query := `INSERT INTO partNok (name, datetime, counter, mkm, video, line) 
	          VALUES (?, ?, ?, ?, ?, ?)`

	_, err := DB.ExecContext(ctx, query, materialCode, time.Now(), counter, mkm, videoFileName, lineName)
	if err != nil {
		logger.Error("[%s] Ошибка сохранения записи брака (counter: %d) в БД: %v", lineName, counter, err)
		return
	}

	logger.Info("[%s] Запись о браке детали %s успешно зафиксирована в БД. Видео: %s",
		lineName, materialCode, videoFileName)
}
