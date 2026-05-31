package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AppConfig описывает структуру нашего файла настроек
type AppConfig struct {
	DbProdConnection     string `json:"db_prod_connection"`
	DbTestConnection     string `json:"db_test_connection"`
	TrassirAddress       string `json:"trassir_address"`
	TrassirUsername      string `json:"trassir_username"`
	TrassirPassword      string `json:"trassir_password"`
	PlcPollingIntervalMs int    `json:"plc_polling_interval_ms"`
	DefaultPrinter       string `json:"DefaultPrinter"`
}

// GlobalConfig хранит загруженные настройки, доступные из любого места программы
var GlobalConfig AppConfig

// LoadConfig читает файл config.json из корня программы
func LoadConfig() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("не удалось получить путь к программе: %w", err)
	}
	appDir := filepath.Dir(execPath)
	configPath := filepath.Join(appDir, "config.json")

	file, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл конфигурации %s: %w", configPath, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&GlobalConfig); err != nil {
		return fmt.Errorf("ошибка парсинга config.json: %w", err)
	}

	return nil
}
