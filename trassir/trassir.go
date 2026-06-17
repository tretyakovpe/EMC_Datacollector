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
	"strings"
	"time"
)

// Инициализируем кастомный HTTP-клиент с правильными настройками TLS
var httpClient = &http.Client{
	Timeout: 120 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
			MaxVersion:         tls.VersionTLS12, // Явно только TLS 1.2
			CipherSuites: []uint16{
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384, // Тот самый шифр из твоего openssl
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			},
		},
	},
}

// LoginResponse структура для парсинга SID
type LoginResponse struct {
	Sid string `json:"sid"`
}

// ExportTaskRequest структура запроса на создание задачи экспорта
type ExportTaskRequest struct {
	ResourceGuid    string `json:"resource_guid"`
	StartTs         int64  `json:"start_ts"`
	EndTs           int64  `json:"end_ts"`
	IsHardware      int    `json:"is_hardware"`
	PreferSubstream int    `json:"prefer_substream"`
}

// ExportTaskResponse структура ответа на создание задачи экспорта
type ExportTaskResponse struct {
	TaskId string `json:"task_id"`
}

// ProcessNokVideoAsync запускается асинхронно в горутине при браке
func ProcessNokVideoAsync(lineName string, cameraGuid string, materialCode string, counter int, mkm []byte) {
	logger.Info("[%s] [VIDEO] Запущен фоновый процесс сохранения брака. Ожидаем 15 сек...", lineName)

	moment := time.Now()
	time.Sleep(15 * time.Second)

	videoFileName, err := saveVideo(lineName, cameraGuid, moment)
	if err != nil {
		logger.Error("[%s] [VIDEO] Сбой обработки видео Трассира: %v. Пишем в базу код '0'", lineName, err)
		videoFileName = "0"
	}

	database.SaveBadPart(lineName, materialCode, counter, mkm, videoFileName)
}

// saveVideo выполняет все 5 шагов запроса и сохранения файла
func saveVideo(lineName string, cameraGuid string, moment time.Time) (string, error) {
	baseURL := config.GlobalConfig.TrassirAddress
	if !strings.HasSuffix(baseURL, "/") {
		baseURL = baseURL + "/"
	}
	logger.Debug("[%s] [VIDEO] Базовый URL: %s", lineName, baseURL)

	// Шаг 1: Получаем session ID (sid)
	loginURL := fmt.Sprintf("%slogin?password=%s", baseURL, config.GlobalConfig.TrassirPassword)
	logger.Debug("[%s] [VIDEO] Авторизация: %s", lineName, loginURL)

	req, err := http.NewRequest("GET", loginURL, nil)
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Connection", "keep-alive")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка запроса авторизации: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	logger.Debug("[%s] [VIDEO] Ответ авторизации: статус=%d, тело=%s", lineName, resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("авторизация вернула статус %d: %s", resp.StatusCode, string(body))
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil || loginResp.Sid == "" {
		return "", fmt.Errorf("не удалось распарсить session ID: %s", string(body))
	}
	sid := loginResp.Sid
	logger.Debug("[%s] [VIDEO] Получен SID: %s", lineName, sid)

	// Шаг 2: Вычисляем временные границы (минус 60 секунд, плюс 30 секунд в микросекундах Unix)
	unixMicrosecondsPerSecond := int64(1000000)
	startTs := (moment.Add(-60 * time.Second).Unix()) * unixMicrosecondsPerSecond
	endTs := (moment.Add(30 * time.Second).Unix()) * unixMicrosecondsPerSecond

	// Преобразуем в человеческий формат для лога
	startTime := time.Unix(startTs/1000000, 0)
	endTime := time.Unix(endTs/1000000, 0)

	logger.Debug("[%s] [VIDEO] Временной диапазон: start=%d (%s), end=%d (%s)",
		lineName,
		startTs,
		startTime.Format("2006-01-02 15:04:05"),
		endTs,
		endTime.Format("2006-01-02 15:04:05"),
	)

	// Шаг 3: Создание задачи экспорта видео
	taskReq := ExportTaskRequest{
		ResourceGuid:    cameraGuid,
		StartTs:         startTs,
		EndTs:           endTs,
		IsHardware:      0,
		PreferSubstream: 0,
	}

	jsonBytes, _ := json.Marshal(taskReq)
	createTaskURL := fmt.Sprintf("%sjit-export-create-task?sid=%s", baseURL, sid)
	logger.Debug("[%s] [VIDEO] Создание задачи: %s", lineName, createTaskURL)

	reqTask, err := http.NewRequest("POST", createTaskURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса задачи: %w", err)
	}
	reqTask.Header.Set("Content-Type", "application/json")
	reqTask.Header.Set("User-Agent", "Mozilla/5.0")

	respTask, err := httpClient.Do(reqTask)
	if err != nil {
		return "", fmt.Errorf("ошибка создания задачи экспорта: %w", err)
	}
	defer respTask.Body.Close()

	bodyTask, _ := io.ReadAll(respTask.Body)
	logger.Debug("[%s] [VIDEO] Ответ создания задачи: статус=%d, тело=%s", lineName, respTask.StatusCode, string(bodyTask))

	if respTask.StatusCode != http.StatusOK {
		return "", fmt.Errorf("создание задачи вернуло статус %d: %s", respTask.StatusCode, string(bodyTask))
	}

	var taskResp ExportTaskResponse
	if err := json.Unmarshal(bodyTask, &taskResp); err != nil || taskResp.TaskId == "" {
		return "", fmt.Errorf("не удалось получить task_id: %s", string(bodyTask))
	}
	taskId := taskResp.TaskId
	logger.Debug("[%s] [VIDEO] Получен task_id: %s", lineName, taskId)

	// Шаг 4: Скачиваем видеофайл
	downloadURL := fmt.Sprintf("%sjit-export-download?sid=%s&task_id=%s", baseURL, sid, taskId)
	logger.Debug("[%s] [VIDEO] Скачивание: %s", lineName, downloadURL)

	reqDownload, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса скачивания: %w", err)
	}
	reqDownload.Header.Set("User-Agent", "Mozilla/5.0")

	respDownload, err := httpClient.Do(reqDownload)
	if err != nil {
		return "", fmt.Errorf("ошибка скачивания видеофайла: %w", err)
	}
	defer respDownload.Body.Close()

	if respDownload.StatusCode != http.StatusOK {
		bodyErr, _ := io.ReadAll(respDownload.Body)
		return "", fmt.Errorf("скачивание вернуло статус %d: %s", respDownload.StatusCode, string(bodyErr))
	}

	videoData, err := io.ReadAll(respDownload.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения видеофайла: %w", err)
	}
	if len(videoData) == 0 {
		return "", fmt.Errorf("видеофайл пуст")
	}
	logger.Debug("[%s] [VIDEO] Скачано %d байт", lineName, len(videoData))

	// Шаг 5: Сохраняем файл
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("не удалось получить путь к программе: %w", err)
	}

	filename := fmt.Sprintf("%s-%s.mp4", lineName, moment.Format("060102150405"))
	appDir := filepath.Dir(execPath)
	videoDir := filepath.Join(appDir, "video")

	if err := os.MkdirAll(videoDir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать папку video: %w", err)
	}

	fullPath := filepath.Join(videoDir, filename)
	if err := os.WriteFile(fullPath, videoData, 0644); err != nil {
		return "", fmt.Errorf("ошибка записи файла на диск: %w", err)
	}

	logger.Info("[%s] [VIDEO] Видео брака сохранено: %s (%d байт)", lineName, filename, len(videoData))
	return filename, nil
}
