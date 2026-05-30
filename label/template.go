package label

// Структуры для 100% совпадения с вашим файлом конфигурации бирок
type LabelTemplate struct {
	Width      float64          `json:"width"`
	Height     float64          `json:"height"`
	Strips     []StripTemplate  `json:"Strips"`
	TextFields []TextTemplate   `json:"TextFields"`
	QRCodes    []QRCodeTemplate `json:"QRCodes"`
}

type StripTemplate struct {
	X1 float64 `json:"X1"`
	Y1 float64 `json:"Y1"`
	X2 float64 `json:"X2"`
	Y2 float64 `json:"Y2"`
}

type TextTemplate struct {
	Name       string  `json:"Name"`
	X          float64 `json:"X"`
	Y          float64 `json:"Y"`
	Width      float64 `json:"Width"`  // Добавили поле Width
	Height     float64 `json:"Height"` // Добавили поле Height
	FontWeight string  `json:"FontWeight"`
	FontSize   int     `json:"FontSize"`
	Format     string  `json:"Format"` // Добавили поле Format
}

type QRCodeTemplate struct {
	Name      string  `json:"Name"`
	X         float64 `json:"X"`
	Y         float64 `json:"Y"`
	Size      float64 `json:"Size"`
	TextBelow float64 `json:"TextBelow"`
	FontSize  int     `json:"FontSize"`
}
