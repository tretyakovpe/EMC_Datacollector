package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var fileLogger *log.Logger
var logFile *os.File

// Init инициализирует логирование в файл внутри папки ~/logs/
func Init() error {
	// Получаем путь к домашней директории пользователя (~/)
	homeDir, err := os.Executable()
	if err != nil {
		return fmt.Errorf("не удалось получить домашнюю директорию: %w", err)
	}
	appDir := filepath.Dir(homeDir)
	// Создаем путь к папке logs
	logsDir := filepath.Join(appDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку logs: %w", err)
	}

	// Формируем имя файла с текущей датой и временем
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logPath := filepath.Join(logsDir, fmt.Sprintf("log_%s.txt", timestamp))

	// Открываем файл на запись (если нет — создаем, пишем в конец)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл лога: %w", err)
	}

	logFile = file
	// Создаем стандартный логгер Go, который пишет в файл с префиксом даты и времени
	fileLogger = log.New(file, "", log.LstdFlags)

	Info("Система логирования успешно запущена. Файл: %s", logPath)
	return nil
}

// Close закрывает файл лога при выходе из программы
func Close() {
	if logFile != nil {
		logFile.Close()
	}
}

// Info пишет информационное сообщение в файл и дублирует в консоль
func Info(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	fullMsg := fmt.Sprintf("[INFO] %s", msg)

	// Вывод в консоль для отладки
	fmt.Println(time.Now().Format("2006/01/02 15:04:05"), fullMsg)

	// Запись в файл
	if fileLogger != nil {
		fileLogger.Println(fullMsg)
	}
}

// Error пишет ошибку в файл и дублирует в консоль
func Error(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	fullMsg := fmt.Sprintf("[ERROR] %s", msg)

	fmt.Fprintln(os.Stderr, time.Now().Format("2006/01/02 15:04:05"), fullMsg)

	if fileLogger != nil {
		fileLogger.Println(fullMsg)
	}
}
