package label

// Структуры конфигурации бирок

type LabelTemplate struct {
	Width           float64             `json:"width"`
	Height          float64             `json:"height"`
	Strips          []StripTemplate     `json:"Strips"`
	TextFields      []TextTemplate      `json:"TextFields"`
	FixedTextFields []FixedTextTemplate `json:"FixedTextFields"`
	QRCodes         []QRCodeTemplate    `json:"QRCodes"`
}

type StripTemplate struct {
	X1 float64 `json:"X1"`
	Y1 float64 `json:"Y1"`
	X2 float64 `json:"X2"`
	Y2 float64 `json:"Y2"`
}

type TextTemplate struct { //Текст который зависит от линии и типа.
	Name       string  `json:"Name"`
	X          float64 `json:"X"`
	Y          float64 `json:"Y"`
	Width      float64 `json:"Width"`
	Height     float64 `json:"Height"`
	FontWeight string  `json:"FontWeight"`
	FontSize   int     `json:"FontSize"`
	Format     string  `json:"Format"`
}

type FixedTextTemplate struct { //Текст одинаковый на каждом экземпляре
	Name         string  `json:"Name"`
	X            float64 `json:"X"`
	Y            float64 `json:"Y"`
	Width        float64 `json:"Width"`
	Height       float64 `json:"Height"`
	FontWeight   string  `json:"FontWeight"`
	FontSize     int     `json:"FontSize"`
	Format       string  `json:"Format"`
	CharsPerLine int     `json:"CharsPerLine"`
	Rows         int     `json:"Rows"`
	Value        string  `json:"Value"`
}

type QRCodeTemplate struct {
	Name      string  `json:"Name"`
	X         float64 `json:"X"`
	Y         float64 `json:"Y"`
	Size      float64 `json:"Size"`
	TextBelow float64 `json:"TextBelow"`
	FontSize  int     `json:"FontSize"`
}
