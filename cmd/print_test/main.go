package main

import (
	"datacollector/label"
	"fmt"
	"time"
)

func main() {
	testBox := label.BoxData{
		LabelCode:      "TEST123456",
		Material:       "C22348-103",
		CustomerNumber: "8450048072",
		Destination:    "08780",
		Description:    "Тестовая бирка для проверки печати",
		Amount:         303,
		Line:           "TEST",
		Date:           time.Now(),
	}

	certPath, err := label.GenerateCertificatePdf(testBox)
	if err != nil {
		fmt.Printf("[%s] Ошибка генерации сертификата: %v", testBox.Line, err)
	} else {
		fmt.Printf("[%s] Сертификат сохранен в: %s", testBox.Line, certPath)
	}
}
