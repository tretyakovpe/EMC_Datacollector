package plc

import (
	"datacollector/logger"

	"github.com/robinson/gos7"
)

// StatusType — тип данных для флагов (аналог enum из C#)
type StatusType int

const (
	Livebit StatusType = iota
	Boxready
	Partready
)

// SetFlagAt записывает управляющие биты в ПЛК в зависимости от типа флага
func SetFlagAt(client gos7.Client, lineName string, flagType StatusType) bool {
	var dbNumber int
	var startByte int
	var bitPos int
	var value bool

	switch flagType {
	case Partready:
		dbNumber = 1013
		startByte = 2
		bitPos = 2
		value = false // Сбрасываем флаг в ПЛК
	case Boxready:
		dbNumber = 1012
		startByte = 1
		bitPos = 0
		value = false // Сбрасываем флаг в ПЛК
	case Livebit:
		dbNumber = 1012
		startByte = 0
		bitPos = 0
		value = true // Взводим Livebit для ПЛК
	default:
		return false
	}

	// Подготавливаем буфер в 1 байт
	buffer := make([]byte, 1)

	// Если мы пишем бит, то по-хорошему нужно сначала считать текущий байт,
	// изменить в нем 1 бит и записать обратно, чтобы не затереть соседние флаги.
	// Но судя по вашему исходному C# коду (S7.SetBitAt(f, 0, x, val)), вы затирали весь байт целиком,
	// так как массив f[1] создавался пустым. Мы повторим эту же логику «в лоб».
	if value {
		buffer[0] = buffer[0] | (1 << bitPos) // Выставляем бит в 1
	} else {
		buffer[0] = buffer[0] &^ (1 << bitPos) // Сбрасываем бит в 0
	}

	// Пишем байт в ПЛК
	err := client.AGWriteDB(dbNumber, startByte, 1, buffer)
	if err != nil {
		logger.Error("[%s] Ошибка записи флага (%d) в DB%d: %v", lineName, flagType, dbNumber, err)
		return false
	}

	return true
}
