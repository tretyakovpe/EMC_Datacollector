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
var LogChannel chan string // НОВОЕ: канал для отправки логов в WebSocket

// Init инициализирует логирование в файл внутри папки ~/logs/
func Init() error {
	// Инициализируем канал (буфер 1000 сообщений)
	LogChannel = make(chan string, 1000)

	homeDir, err := os.Executable()
	if err != nil {
		return fmt.Errorf("не удалось получить домашнюю директорию: %w", err)
	}
	appDir := filepath.Dir(homeDir)
	logsDir := filepath.Join(appDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку logs: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logPath := filepath.Join(logsDir, fmt.Sprintf("log_%s.txt", timestamp))

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл лога: %w", err)
	}

	logFile = file
	fileLogger = log.New(file, "", log.LstdFlags)

	Info("Система логирования успешно запущена. Файл: %s", logPath)
	return nil
}

// Close закрывает файл лога
func Close() {
	if logFile != nil {
		logFile.Close()
	}
}

// sendToChannel отправляет сообщение в WebSocket-канал (неблокирующе)
func sendToChannel(level, msg string) {
	if LogChannel == nil {
		return
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	select {
	case LogChannel <- fmt.Sprintf(`{"time":"%s","level":"%s","message":"%s"}`, timestamp, level, escapeJSON(msg)):
	default:
		// Канал переполнен — пропускаем, не блокируем основную логику
	}
}

// escapeJSON экранирует кавычки и спецсимволы для JSON
func escapeJSON(s string) string {
	result := ""
	for _, c := range s {
		switch c {
		case '"':
			result += "\\\""
		case '\\':
			result += "\\\\"
		case '\n':
			result += "\\n"
		case '\r':
			result += "\\r"
		case '\t':
			result += "\\t"
		default:
			result += string(c)
		}
	}
	return result
}

// Info пишет информационное сообщение
func Info(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	fullMsg := fmt.Sprintf("[INFO] %s", msg)

	fmt.Println(time.Now().Format("2006/01/02 15:04:05"), fullMsg)

	if fileLogger != nil {
		fileLogger.Println(fullMsg)
	}

	// НОВОЕ: отправляем в WebSocket-канал
	sendToChannel("INFO", msg)
}

// Error пишет ошибку
func Error(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	fullMsg := fmt.Sprintf("[ERROR] %s", msg)

	fmt.Fprintln(os.Stderr, time.Now().Format("2006/01/02 15:04:05"), fullMsg)

	if fileLogger != nil {
		fileLogger.Println(fullMsg)
	}

	// НОВОЕ: отправляем в WebSocket-канал
	sendToChannel("ERROR", msg)
}

// Debug пишет отладочное сообщение (только если включён режим отладки)
func Debug(format string, v ...interface{}) {
	if LogChannel == nil {
		return
	}
	msg := fmt.Sprintf(format, v...)
	fullMsg := fmt.Sprintf("[DEBUG] %s", msg)

	fmt.Println(time.Now().Format("2006/01/02 15:04:05"), fullMsg)

	if fileLogger != nil {
		fileLogger.Println(fullMsg)
	}

	sendToChannel("DEBUG", msg)
}
