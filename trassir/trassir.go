package trassir

import (
	"bytes"
	"crypto/tls"
	"datacollector/config"
	"datacollector/database"
	"datacollector/logger"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Инициализируем кастомный HTTP-клиент, который полностью игнорирует проверку SSL-сертификатов
var httpClient = &http.Client{
	Timeout: 60 * time.Second, // Долгий таймаут, так как видео генерируется не мгновенно
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Аналог ServerCertificateCustomValidationCallback = true
	},
}

// Структура для парсинга SID авторизации
type LoginResponse struct {
	Sid string `json:"sid"`
}

// Структура запроса на создание задачи экспорта
type ExportTaskRequest struct {
	ResourceGuid    string `json:"resource_guid"`
	StartTs         int64  `json:"start_ts"`
	EndTs           int64  `json:"end_ts"`
	IsHardware      int    `json:"is_hardware"`
	PreferSubstream int    `json:"prefer_substream"`
}

// Структура ответа на создание задачи экспорта
type ExportTaskResponse struct {
	TaskId string `json:"task_id"`
}

// ProcessNokVideoAsync запускается асинхронно в горутине при браке
func ProcessNokVideoAsync(lineName string, cameraGuid string, materialCode string, counter int, mkm []byte) {
	logger.Info("[%s] [VIDEO] Запущен фоновый процесс сохранения брака. Ожидаем 15 сек...", lineName)

	// Засыпаем на 15 секунд, давая Трассиру физически дописать буфер видео
	time.Sleep(15 * time.Second)

	moment := time.Now()
	videoFileName, err := saveVideo(lineName, cameraGuid, moment)
	if err != nil {
		logger.Error("[%s] [VIDEO] Сбой обработки видео Трассира: %v. Пишем в базу код '0'", lineName, err)
		videoFileName = "0"
	}

	// Фиксируем лог в БД
	database.SaveBadPart(lineName, materialCode, counter, mkm, videoFileName)
}

// saveVideo выполняет все 5 шагов запроса и сохранения файла
func saveVideo(lineName string, cameraGuid string, moment time.Time) (string, error) {
	// Шаг 1: Получаем session ID (sid)
	loginUrl := fmt.Sprintf("%s/login?password=%s", config.GlobalConfig.TrassirAddress, config.GlobalConfig.TrassirPassword)
	resp, err := httpClient.Get(loginUrl)
	if err != nil {
		return "", fmt.Errorf("ошибка запроса авторизации: %w", err)
	}
	defer resp.Body.Close()

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil || loginResp.Sid == "" {
		return "", fmt.Errorf("не удалось распарсить session ID (sid)")
	}
	sid := loginResp.Sid

	// Шаг 2: Вычисляем временные границы (минус 60 секунд, плюс 30 секунд в микросекундах Unix)
	unixMicrosecondsPerSecond := int64(1000000)

	startTs := (moment.Add(-60 * time.Second).Unix()) * unixMicrosecondsPerSecond
	endTs := (moment.Add(30 * time.Second).Unix()) * unixMicrosecondsPerSecond

	// Шаг 3: Создание задачи экспорта видео
	taskReq := ExportTaskRequest{
		ResourceGuid:    cameraGuid,
		StartTs:         startTs,
		EndTs:           endTs,
		IsHardware:      0,
		PreferSubstream: 0,
	}

	jsonBytes, _ := json.Marshal(taskReq)
	createTaskUrl := fmt.Sprintf("%sjit-export-create-task?sid=%s", config.GlobalConfig.TrassirAddress, sid)

	respTask, err := httpClient.Post(createTaskUrl, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", fmt.Errorf("ошибка создания задачи экспорта: %w", err)
	}
	defer respTask.Body.Close()

	var taskResp ExportTaskResponse
	if err := json.NewDecoder(respTask.Body).Decode(&taskResp); err != nil || taskResp.TaskId == "" {
		return "", fmt.Errorf("не удалось получить task_id")
	}

	// Шаг 4: Скачиваем видеофайл
	downloadUrl := fmt.Sprintf("%sjit-export-download?sid=%s&task_id=%s", config.GlobalConfig.TrassirAddress, sid, taskResp.TaskId)
	respDownload, err := httpClient.Get(downloadUrl)
	if err != nil {
		return "", fmt.Errorf("ошибка скачивания видеофайла: %w", err)
	}
	defer respDownload.Body.Close()

	videoData, err := io.ReadAll(respDownload.Body)
	if err != nil || len(videoData) == 0 {
		return "", fmt.Errorf("видеофайл пуст или не скачался")
	}

	// Шаг 5: Сохраняем файл в корень программы в папку video
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("не удалось получить путь к программе: %w", err)
	}

	filename := fmt.Sprintf("%s-%s.mp4", lineName, moment.Format("060102150405"))
	appDir := filepath.Dir(execPath)
	videoDir := filepath.Join(appDir, "video")

	// Создаем папку, если её нет (аналог Directory.CreateDirectory)
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать папку video: %w", err)
	}

	fullPath := filepath.Join(videoDir, filename)
	if err := os.WriteFile(fullPath, videoData, 0644); err != nil {
		return "", fmt.Errorf("ошибка записи файла на диск: %w", err)
	}

	logger.Info("[%s] [VIDEO] Видео брака успешно скачано и сохранено: %s", lineName, filename)
	return filename, nil
}
