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
	LabelCode   string
	Material    string
	Description string
	Amount      int
	Line        string
	Shift       string
	Date        time.Time
}

// getValueByName обрабатывает имя поля и применяет маску форматирования из JSON (C# style)
func getValueByName(name string, format string, box BoxData) string {
	switch name {
	case "MaterialCode", "Material":
		return box.Material
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
		// Замена C# маски "hh\:mm\:ss" на формат времени Go
		return box.Date.Format("15:04:05")
	default:
		return ""
	}
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

		// Специальная логика автопереноса для длинного поля Description (названия детали)
		if textField.Name == "Description" && len(val) > 25 && textField.Width > 0 {
			// Бьем текст на две строки по пробелу, чтобы уложить в Width: 90
			words := strings.Split(val, " ")
			line1, line2 := "", ""
			for _, word := range words {
				if len(line1)+len(word) < 22 {
					line1 += word + " "
				} else {
					line2 += word + " "
				}
			}
			// Выводим первую строчку
			svg.WriteString(fmt.Sprintf("<text x='%f' y='%f' text-anchor='start' dominant-baseline='text-before-edge' font-weight='%s' font-size='%d'>%s</text>\n",
				textField.X, textField.Y, textField.FontWeight, textField.FontSize, strings.TrimSpace(line1)))
			// Выводим вторую строчку со смещением вниз на размер шрифта + 2px
			svg.WriteString(fmt.Sprintf("<text x='%f' y='%f' text-anchor='start' dominant-baseline='text-before-edge' font-weight='%s' font-size='%d'>%s</text>\n",
				textField.X, textField.Y+float64(textField.FontSize)+2, textField.FontWeight, textField.FontSize, strings.TrimSpace(line2)))
		} else {
			// Обычный однострочный вывод текста
			svg.WriteString(fmt.Sprintf("<text id='%s' x='%f' y='%f' text-anchor='start' dominant-baseline='text-before-edge' font-weight='%s' font-size='%d'>%s</text>\n",
				textField.Name, textField.X, textField.Y, textField.FontWeight, textField.FontSize, val))
		}
	}

	// --- РИСУЕМ ВЕКТОРНЫЕ QR-КОДЫ ---
	for _, qrTmp := range template.QRCodes {
		qrValue := fmt.Sprintf("3OS%s%s", box.Material, box.LabelCode)

		// 1. Генерируем матрицу QR-кода пакетом barcode (автор boombuler)
		qrCode, err := qr.Encode(qrValue, qr.Q, qr.Auto)
		if err != nil {
			return "", fmt.Errorf("ошибка генерации QR матрицы: %w", err)
		}

		// 2. Рассчитываем реальный размер на листе А5 (Size из JSON * 22.5 для масштаба)
		visualSize := qrTmp.Size
		if visualSize <= 5 {
			visualSize = qrTmp.Size * 22.5
		}

		// Получаем количество модулей (точек) в QR-коде
		bounds := qrCode.Bounds()
		qrWidth := bounds.Max.X - bounds.Min.X
		qrHeight := bounds.Max.Y - bounds.Min.Y

		// Рассчитываем физический размер одной точки QR-кода в миллиметрах/пикселях холста
		moduleSize := visualSize / float64(qrWidth)

		// Открываем группу тегов SVG для QR-кода сдвигом на нужные координаты (X, Y)
		svg.WriteString(fmt.Sprintf("<g transform='translate(%f, %f)'>\n", qrTmp.X, qrTmp.Y))
		// Рисуем белый фоновый квадрат, чтобы линии под QR-кодом не просвечивали
		svg.WriteString(fmt.Sprintf("  <rect x='0' y='0' width='%f' height='%f' fill='white' />\n", visualSize, visualSize))

		// Циклом обходим матрицу QR-кода и рисуем черные квадратики через векторные <rect>
		for x := 0; x < qrWidth; x++ {
			for y := 0; y < qrHeight; y++ {
				// Метод RGBA() возвращает 4 значения (r, g, b, a)
				r, _, _, _ := qrCode.At(x, y).RGBA()

				// В пакете barcode черные пиксели имеют значение 0
				if r == 0 {
					svg.WriteString(fmt.Sprintf("  <rect x='%f' y='%f' width='%f' height='%f' fill='black' />\n",
						float64(x)*moduleSize, float64(y)*moduleSize, moduleSize+0.05, moduleSize+0.05))
				}
			}
		}
		svg.WriteString("</g>\n")
		// 3. Текст под QR-кодом (позиция Y + TextBelow)
		svg.WriteString(fmt.Sprintf("<text x='%f' y='%f' text-anchor='start' dominant-baseline='text-before-edge' font-size='%d'>%s</text>\n",
			qrTmp.X, qrTmp.Y+qrTmp.TextBelow, qrTmp.FontSize, qrValue))
	}

	svg.WriteString("</svg>")

	// --- АВТОМАТИЧЕСКОЕ СОХРАНЕНИЕ СЫРОГО SVG НА ДИСК ДЛЯ ПРЕДПРОСМОТРА ---
	svgDir := filepath.Join(appDir, "svg_debug")
	_ = os.MkdirAll(svgDir, 0755)
	svgPath := filepath.Join(svgDir, fmt.Sprintf("%s.svg", box.LabelCode))
	_ = os.WriteFile(svgPath, svg.Bytes(), 0644) // Сохраняем как чистый .svg файл!
	// ---------------------------------------------------------------------

	// 3. Рендеринг итогового PDF-файла (пакет canvas, автор tdewolff)
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

	// Переводим физические размеры А5 из миллиметров в пункты PDF (96 точки на дюйм)
	// дюйм = 25.4 мм. Значит 1 мм = 96 / 25.4 пунктов.
	mmToPoints := 96.0 / 25.4
	pdfWidthPoints := template.Width * mmToPoints
	pdfHeightPoints := template.Height * mmToPoints

	// 2. Инициализируем PDF-рендерер в точных размерах листа
	pdfRenderer := pdf.New(pdfFile, pdfWidthPoints, pdfHeightPoints, nil)

	// 3. Создаем контекст рисования (Canvas) поверх PDF-файла.
	// Это автоматически решает проблему переворота оси Y (из SVG-верх в PDF-низ)
	// и подгоняет масштаб один к одному.
	ctx := canvas.NewContext(pdfRenderer)

	// Устанавливаем масштаб контекста: переводим пиксели SVG в миллиметры PDF
	ctx.SetCoordSystem(canvas.CartesianIV) // Включает систему координат SVG (0,0 сверху-слева)
	ctx.Scale(mmToPoints, mmToPoints)      // Масштабируем единицы в миллиметры

	// 4. Отрисовываем наш векторный холст SVG внутрь настроенного контекста
	c.RenderTo(ctx)

	// Финализируем и закрываем файл
	if err := pdfRenderer.Close(); err != nil {
		return "", fmt.Errorf("ошибка финализации PDF: %w", err)
	}

	logger.Info("[%s] Динамический PDF по JSON-шаблону '%s' успешно сгенерирован ровно 1:1 (в мм).", box.Line, labelType)
	return pdfPath, nil
}

func bytesToBase64(b []byte) string {
	const to64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var buf bytes.Buffer
	for i := 0; i < len(b); i += 3 {
		var relen = len(b) - i
		if relen > 3 {
			relen = 3
		}
		var chunk = uint32(0)
		for j := 0; j < relen; j++ {
			chunk |= uint32(b[i+j]) << (16 - uint32(j)*8)
		}
		for j := 0; j < relen+1; j++ {
			buf.WriteByte(to64[(chunk>>(18-uint32(j)*6))&0x3F])
		}
		for j := 0; j < 3-relen; j++ {
			buf.WriteByte('=')
		}
	}
	return buf.String()
}
