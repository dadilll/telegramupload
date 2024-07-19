package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/zelenin/go-tdlib/client"
)

// Initialize Tdlib client
var tdlibClient *client.Client

func main() {
	// Создание клиента Tdlib
	var err error
	tdlibClient, err = client.NewClient(client.Config{
		APIID:              "your_api_id",
		APIHash:            "your_api_hash",
		SystemLanguageCode: "en",
		DeviceModel:        "Desktop",
		SystemVersion:      "Unknown",
		ApplicationVersion: "1.0",
		DatabaseDirectory:  "./tdlib-db",
		FileDirectory:      "./tdlib-files",
	})
	if err != nil {
		log.Fatalf("Ошибка создания клиента tdlib: %v", err)
	}

	// Авторизация
	_, err = tdlibClient.Authorize()
	if err != nil {
		log.Fatalf("Ошибка авторизации: %v", err)
	}

	// Запуск HTTP сервера
	http.HandleFunc("/upload", handleFileUpload)
	log.Println("Сервер запущен на порту 8080...")
	http.ListenAndServe(":8080", nil)
}

func handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Только POST метод поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Получение файла из формы
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Ошибка получения файла", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Сохранение файла на диск
	filePath := filepath.Join("./uploads", header.Filename)
	out, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Ошибка сохранения файла", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		http.Error(w, "Ошибка копирования файла", http.StatusInternalServerError)
		return
	}

	// Загрузка файла на сервер Telegram
	uploadedFile, err := tdlibClient.UploadFile(filePath)
	if err != nil {
		http.Error(w, "Ошибка загрузки файла на Telegram", http.StatusInternalServerError)
		return
	}

	// Возвращение file_id обратно боту
	w.Write([]byte(uploadedFile.Remote.FileID))
	fmt.Printf("Файл успешно загружен с ID: %v\n", uploadedFile.Remote.FileID)
}
