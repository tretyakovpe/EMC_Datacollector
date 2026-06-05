package label

import (
	"bytes"
	"datacollector/logger"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Автор: boombuler, Пакет: barcode
	"github.com/boombuler/barcode/qr"

	// Автор: tdewolff, Пакет: canvas
	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/pdf"
)

// BoxData описывает все данные, которые выводятся на бирку
type BoxData struct {
	LabelCode      string
	Material       string
	Destination    string
	CustomerNumber string
	Description    string
	Amount         int
	Line           string
	Shift          string
	Date           time.Time
}

// getValueByName обрабатывает имя поля и применяет маску форматирования из JSON (C# style)
func getValueByName(name string, format string, box BoxData) string {
	switch name {
	case "MaterialCode", "Material":
		return box.Material
	case "Destination":
		return box.Destination
	case "CustomerNumber", "CustomerCode":
		return box.CustomerNumber
	case "Description":
		return box.Description
	case "Amount":
		return fmt.Sprintf("%d шт.", box.Amount)
	case "Label", "LabelCode", "HUNumber", "labelNumber":
		return box.LabelCode
	case "Line", "Name":
		return box.Line
	case "Shift":
		return box.Shift
	case "Date", "Datetime":
		if format == "dd:MM:yyyy" {
			return box.Date.Format("02.01.2006")
		}
		return box.Date.Format("02.01.2006")
	case "Time":
		return box.Date.Format("15:04:05")
	default:
		return ""
	}
}

// wrapText разбивает текст на строки заданной максимальной длины
// старается разбивать по пробелам, не разрывая слова
func wrapText(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	currentLine := ""

	for _, word := range words {
		// Если слово само длиннее maxLen, разбиваем принудительно
		if len(word) > maxLen {
			if currentLine != "" {
				lines = append(lines, strings.TrimSpace(currentLine))
				currentLine = ""
			}
			// Разбиваем длинное слово на части
			for i := 0; i < len(word); i += maxLen {
				end := i + maxLen
				if end > len(word) {
					end = len(word)
				}
				lines = append(lines, word[i:end])
			}
			continue
		}

		// Проверяем, поместится ли слово в текущую строку
		if len(currentLine)+len(word)+1 > maxLen {
			if currentLine != "" {
				lines = append(lines, strings.TrimSpace(currentLine))
				currentLine = ""
			}
		}
		currentLine += word + " "
	}

	if currentLine != "" {
		lines = append(lines, strings.TrimSpace(currentLine))
	}

	return lines
}

// escapeXml экранирует XML спецсимволы
func escapeXml(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// GenerateLabelPdf парсит JSON-шаблон, собирает SVG с QR-кодом и делает PDF
func GenerateLabelPdf(box BoxData, labelType string) (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	appDir := filepath.Dir(execPath)

	// 1. Читаем JSON-файл разметки
	jsonPath := filepath.Join(appDir, "Labels", fmt.Sprintf("%s.json", labelType))
	jsonBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		return "", fmt.Errorf("файл шаблона %s не найден: %w", jsonPath, err)
	}

	var template LabelTemplate
	if err := json.Unmarshal(jsonBytes, &template); err != nil {
		return "", fmt.Errorf("ошибка парсинга структуры JSON: %w", err)
	}

	// 2. Сборка SVG в памяти на основе координат из шаблона
	var svg bytes.Buffer
	svg.WriteString(fmt.Sprintf("<svg width='%fmm' height='%fmm' viewBox='0 0 %f %f' version='1.1' xmlns='http://www.w3.org/2000/svg'>\n",
		template.Width, template.Height, template.Width, template.Height))
	svg.WriteString("<style> text {font-family: sans-serif;}</style>\n")

	// Рисуем линии (Strips)
	for _, line := range template.Strips {
		svg.WriteString(fmt.Sprintf("<line x1='%f' y1='%f' x2='%f' y2='%f' stroke='black' stroke-width='1' />\n",
			line.X1, line.Y1, line.X2, line.Y2))
	}

	// Рисуем текстовые поля (TextFields)
	for _, textField := range template.TextFields {
		val := getValueByName(textField.Name, textField.Format, box)
		if val == "" {
			continue
		}

		// Экранируем XML спецсимволы
		val = escapeXml(val)

		// Специальная логика автопереноса для длинного поля Description
		if textField.Name == "Description" && len(val) > 30 && textField.Width > 0 {
			// Разбиваем текст на строки (максимум 25 символов)
			lines := wrapText(val, 30)

			for i, line := range lines {
				if i >= 2 { // максимум 2 строки для бирки
					break
				}
				// Смещение по Y: первая строка на месте, вторая — ниже
				yOffset := textField.Y + float64(i*(textField.FontSize+2))
				svg.WriteString(fmt.Sprintf("<text x='%f' y='%f' text-anchor='start' dominant-baseline='text-before-edge' font-weight='%s' font-size='%d'>%s</text>\n",
					textField.X, yOffset, textField.FontWeight, textField.FontSize, line))
			}
		} else {
			// Обычный однострочный вывод текста
			svg.WriteString(fmt.Sprintf("<text id='%s' x='%f' y='%f' text-anchor='start' dominant-baseline='text-before-edge' font-weight='%s' font-size='%d'>%s</text>\n",
				textField.Name, textField.X, textField.Y, textField.FontWeight, textField.FontSize, val))
		}
	}

	// --- РИСУЕМ ВЕКТОРНЫЕ QR-КОДЫ ---
	for _, qrTmp := range template.QRCodes {
		qrValue := fmt.Sprintf("3OS%s%s", box.Material, box.LabelCode)

		// 1. Генерируем матрицу QR-кода
		qrCode, err := qr.Encode(qrValue, qr.Q, qr.Auto)
		if err != nil {
			return "", fmt.Errorf("ошибка генерации QR матрицы: %w", err)
		}

		// 2. Рассчитываем реальный размер на листе
		visualSize := qrTmp.Size
		if visualSize <= 5 {
			visualSize = qrTmp.Size * 22.5
		}

		bounds := qrCode.Bounds()
		qrWidth := bounds.Max.X - bounds.Min.X
		qrHeight := bounds.Max.Y - bounds.Min.Y
		moduleSize := visualSize / float64(qrWidth)

		svg.WriteString(fmt.Sprintf("<g transform='translate(%f, %f)'>\n", qrTmp.X, qrTmp.Y))
		svg.WriteString(fmt.Sprintf("  <rect x='0' y='0' width='%f' height='%f' fill='white' />\n", visualSize, visualSize))

		for x := 0; x < qrWidth; x++ {
			for y := 0; y < qrHeight; y++ {
				r, _, _, _ := qrCode.At(x, y).RGBA()
				if r == 0 {
					svg.WriteString(fmt.Sprintf("  <rect x='%f' y='%f' width='%f' height='%f' fill='black' />\n",
						float64(x)*moduleSize, float64(y)*moduleSize, moduleSize+0.05, moduleSize+0.05))
				}
			}
		}
		svg.WriteString("</g>\n")

		// Текст под QR-кодом
		svg.WriteString(fmt.Sprintf("<text x='%f' y='%f' text-anchor='start' dominant-baseline='text-before-edge' font-size='%d'>%s</text>\n",
			qrTmp.X, qrTmp.Y+qrTmp.TextBelow, qrTmp.FontSize, escapeXml(qrValue)))
	}

	svg.WriteString("</svg>")

	// Сохраняем SVG для отладки
	svgDir := filepath.Join(appDir, "svg_debug")
	_ = os.MkdirAll(svgDir, 0755)
	svgPath := filepath.Join(svgDir, fmt.Sprintf("%s.svg", box.LabelCode))
	_ = os.WriteFile(svgPath, svg.Bytes(), 0644)

	// 3. Рендеринг PDF
	pdfDir := filepath.Join(appDir, "PDF")
	_ = os.MkdirAll(pdfDir, 0755)
	pdfPath := filepath.Join(pdfDir, fmt.Sprintf("%s.pdf", box.LabelCode))

	c, err := canvas.ParseSVG(&svg)
	if err != nil {
		return "", fmt.Errorf("ошибка компиляции SVG в холст: %w", err)
	}

	pdfFile, err := os.Create(pdfPath)
	if err != nil {
		return "", err
	}
	defer pdfFile.Close()

	mmToPoints := 96.0 / 25.4
	pdfWidthPoints := template.Width * mmToPoints
	pdfHeightPoints := template.Height * mmToPoints

	pdfRenderer := pdf.New(pdfFile, pdfWidthPoints, pdfHeightPoints, nil)
	ctx := canvas.NewContext(pdfRenderer)
	ctx.SetCoordSystem(canvas.CartesianIV)
	ctx.Scale(mmToPoints, mmToPoints)
	c.RenderTo(ctx)

	if err := pdfRenderer.Close(); err != nil {
		return "", fmt.Errorf("ошибка финализации PDF: %w", err)
	}

	logger.Info("[%s] Динамический PDF по JSON-шаблону '%s' успешно сгенерирован", box.Line, labelType)
	return pdfPath, nil
}
