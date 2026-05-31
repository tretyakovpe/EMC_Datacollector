package webserver

import (
	"datacollector/database"
	"datacollector/label"
	"datacollector/logger"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ========== Глобальные переменные ==========
var (
	upgrader = websocket.Upgrader{
		CheckOrigin:     func(r *http.Request) bool { return true },
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	globalHub *Hub
)

// ========== WebSocket Hub ==========
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) BroadcastMessage(msgType string, data interface{}) {
	msg := map[string]interface{}{
		"type": msgType,
		"data": data,
		"ts":   time.Now().Unix(),
	}
	jsonMsg, _ := json.Marshal(msg)
	h.broadcast <- jsonMsg
}

func (h *Hub) ServeWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &Client{hub: h, conn: conn, send: make(chan []byte, 256)}
	h.register <- client

	// write pump
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer func() {
			ticker.Stop()
			client.conn.Close()
		}()

		for {
			select {
			case message, ok := <-client.send:
				if !ok {
					client.conn.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}
				client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
					return
				}
			case <-ticker.C:
				client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// read pump
	go func() {
		defer func() {
			h.unregister <- client
			client.conn.Close()
		}()

		for {
			_, _, err := client.conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

// ========== HTTP Server ==========
func StartServer(debugMode bool, logChan <-chan string) {
	globalHub = NewHub()
	go globalHub.Run()

	// Пересылка логов в WebSocket
	go func() {
		for msg := range logChan {
			var logData map[string]string
			if err := json.Unmarshal([]byte(msg), &logData); err == nil {
				globalHub.BroadcastMessage("log", logData)
			}
		}
	}()

	port := "80"
	if debugMode {
		port = "8080"
	}

	// Получаем путь к исполняемому файлу
	exePath, err := os.Executable()
	if err != nil {
		logger.Error("Не удалось определить путь к программе: %v", err)
		return
	}
	rootDir := filepath.Dir(exePath)
	staticDir := filepath.Join(rootDir, "wwwroot")

	mux := http.NewServeMux()

	// Раздача CSS файлов
	mux.HandleFunc("/css/", func(w http.ResponseWriter, r *http.Request) {
		fullPath := filepath.Join(staticDir, r.URL.Path)

		// Проверка безопасности
		if !strings.HasPrefix(fullPath, staticDir) {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Del("X-Content-Type-Options")
		http.ServeFile(w, r, fullPath)
	})

	// Раздача JS файлов
	mux.HandleFunc("/js/", func(w http.ResponseWriter, r *http.Request) {
		fullPath := filepath.Join(staticDir, r.URL.Path)

		if !strings.HasPrefix(fullPath, staticDir) {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Del("X-Content-Type-Options")
		http.ServeFile(w, r, fullPath)
	})

	// Раздача изображений
	mux.HandleFunc("/images/", func(w http.ResponseWriter, r *http.Request) {
		fullPath := filepath.Join(staticDir, r.URL.Path)

		if !strings.HasPrefix(fullPath, staticDir) {
			http.NotFound(w, r)
			return
		}

		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}

		// Определяем MIME тип по расширению
		ext := filepath.Ext(fullPath)
		switch ext {
		case ".png":
			w.Header().Set("Content-Type", "image/png")
		case ".jpg", ".jpeg":
			w.Header().Set("Content-Type", "image/jpeg")
		case ".svg":
			w.Header().Set("Content-Type", "image/svg+xml")
		case ".ico":
			w.Header().Set("Content-Type", "image/x-icon")
		}

		w.Header().Del("X-Content-Type-Options")
		http.ServeFile(w, r, fullPath)
	})

	// Корневой маршрут
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Del("X-Content-Type-Options")
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}
		http.NotFound(w, r)
	})

	// WebSocket
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		globalHub.ServeWs(w, r)
	})

	// Раздача конфигов
	mux.HandleFunc("/config/", func(w http.ResponseWriter, r *http.Request) {
		fullPath := filepath.Join(rootDir, r.URL.Path)

		if !strings.HasPrefix(fullPath, rootDir) {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Del("X-Content-Type-Options")
		http.ServeFile(w, r, fullPath)
	})

	// REST API
	mux.HandleFunc("/api/lines", handleGetLines)
	mux.HandleFunc("/api/current-box/", handleGetCurrentBox)
	mux.HandleFunc("/api/stats", handleGetStats)
	mux.HandleFunc("/api/reprint-label/", handleReprintLabel)
	// Изменить статус линии (вкл/выкл)
	mux.HandleFunc("/api/lines/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			// POST /api/lines/{lineName}/status - изменить статус
			handleSetLineStatus(w, r)
		case http.MethodGet:
			handleGetLineStatus(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	addr := "0.0.0.0:" + port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("Не удалось запустить веб-сервер на порту %s: %v", port, err)
		return
	}

	logger.Info("=== ВЕБ-СЕРВЕР ЗАПУЩЕН ===")
	logger.Info("Адрес: http://%s", addr)
	logger.Info("Статика: %s", staticDir)
	logger.Info("Режим: %s", map[bool]string{true: "ОТЛАДКА (тестовая БД)", false: "БОЕВОЙ"}[debugMode])

	go func() {
		if err := http.Serve(listener, mux); err != nil {
			logger.Error("Веб-сервер остановлен: %v", err)
		}
	}()
}

// ========== REST Handlers ==========
func handleGetLines(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	lines, err := database.GetLinesStatusForAPI()
	if err != nil {
		logger.Error("API /api/lines: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lines)
}

func handleGetCurrentBox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Missing line name", http.StatusBadRequest)
		return
	}
	lineName := parts[3]

	boxInfo, err := database.GetCurrentBoxInfo(lineName)
	if err != nil {
		logger.Error("API /api/current-box/%s: %v", lineName, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if boxInfo == nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "no_active_box"})
		return
	}
	json.NewEncoder(w).Encode(boxInfo)
}

func handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := database.GetTodaysBoxesCount()
	if err != nil {
		logger.Error("API /api/stats: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func handleReprintLabel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Missing label ID", http.StatusBadRequest)
		return
	}
	labelID := parts[3]

	pdfPath, err := database.FindLabelPDFPath(labelID)
	if err != nil {
		logger.Error("API перепечатки: %v", err)
		http.Error(w, "Label PDF not found", http.StatusNotFound)
		return
	}

	// TODO: получить принтер из БД по линии
	printer := "togp0004.emc-tlt.tech"

	if err := label.PrintLabelNetwork(pdfPath, printer, "API"); err != nil {
		logger.Error("Ошибка перепечатки бирки %s: %v", labelID, err)
		http.Error(w, "Print failed", http.StatusInternalServerError)
		return
	}

	logger.Info("Успешная перепечатка бирки %s на принтере %s", labelID, printer)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Reprint sent to printer",
		"labelId": labelID,
	})
}

// handleSetLineStatus - установить статус линии (вкл/выкл)
func handleSetLineStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Пытаемся получить имя линии из query параметра
	lineName := r.URL.Query().Get("name")

	// Парсим тело запроса
	var req struct {
		IsOnline bool `json:"isOnline"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Обновляем статус в БД
	database.UpdateLineOnlineStatus(lineName, req.IsOnline)

	logger.Info("[API] Статус линии %s изменён на: %v", lineName, req.IsOnline)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"message":  fmt.Sprintf("Line %s status changed to %v", lineName, req.IsOnline),
		"line":     lineName,
		"isOnline": req.IsOnline,
	})
}

// handleGetLineStatus - получить статус линии
func handleGetLineStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Missing line name", http.StatusBadRequest)
		return
	}
	lineName := parts[2]

	// Получаем статус из БД
	status, err := database.GetLineOnlineStatus(lineName)
	if err != nil {
		logger.Error("API /api/lines/%s/status: %v", lineName, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"line":     lineName,
		"isOnline": status,
	})
}
