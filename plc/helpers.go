package plc

import (
	"encoding/binary"
	"math"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// GetBitAt возвращает true, если бит на позиции bitPos в байте bytePos равен 1
func GetBitAt(buffer []byte, bytePos int, bitPos int) bool {
	if bytePos >= len(buffer) {
		return false
	}
	return (buffer[bytePos] & (1 << bitPos)) != 0
}

// GetIntAt читает 16-битное целое число (S7 Int, 2 байта) в формате BigEndian
func GetIntAt(buffer []byte, pos int) int {
	if pos+2 > len(buffer) {
		return 0
	}
	return int(binary.BigEndian.Uint16(buffer[pos : pos+2]))
}

// GetRealAt читает 32-битное число с плавающей точкой (S7 Real, 4 байта)
func GetRealAt(buffer []byte, pos int) float64 {
	if pos+4 > len(buffer) {
		return 0.0
	}
	bits := binary.BigEndian.Uint32(buffer[pos : pos+4])
	return float64(math.Float32frombits(bits))
}

// GetStringAt читает стандартную строку Siemens S7 String.
// Первые два байта — это MaxLength и CurrentLength, сами символы идут дальше.
func GetStringAt(buffer []byte, pos int) string {
	if pos+2 > len(buffer) {
		return ""
	}
	// В S7 строках второй байт указывает на реальную длину текста
	length := int(buffer[pos+1])
	if pos+2+length > len(buffer) {
		return ""
	}

	rawBytes := buffer[pos+2 : pos+2+length]
	return strings.TrimSpace(string(rawBytes))
}

// GetWin1251String читает сырой кусок байтов ASCII/Windows-1251 заданной длины
// и корректно переводит его в UTF-8 строку для Linux.
func GetWin1251String(buffer []byte, pos int, length int) string {
	if pos+length > len(buffer) {
		return ""
	}
	rawBytes := buffer[pos : pos+length]

	// Конвертируем Windows-1251 в UTF-8
	decoder := charmap.Windows1251.NewDecoder()
	utf8Bytes, err := decoder.Bytes(rawBytes)
	if err != nil {
		return strings.TrimSpace(string(rawBytes)) // Если сбой — отдаем как есть
	}

	return strings.TrimSpace(string(utf8Bytes))
}
