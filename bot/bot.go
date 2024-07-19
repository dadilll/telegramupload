package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func main() {
	bot, err := tgbotapi.NewBotAPI("your_bot_token")
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.Document != nil {
			handleIncomingDocument(bot, update.Message)
		}
	}
}

func handleIncomingDocument(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	fileID := message.Document.FileID

	file, err := bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		log.Printf("Ошибка получения файла: %v", err)
		return
	}

	// Загрузка файла на локальную систему
	filePath := "downloads/" + message.Document.FileName
	err = downloadFile(file.Link(bot.Token), filePath)
	if err != nil {
		log.Printf("Ошибка загрузки файла: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Ошибка загрузки файла.")
		bot.Send(msg)
		return
	}

	// Отправка файла агенту
	fileIDFromAgent, err := uploadFileToAgent(filePath)
	if err != nil {
		log.Printf("Ошибка отправки файла агенту: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Ошибка отправки файла агенту.")
		bot.Send(msg)
		return
	}

	// Уведомление пользователя об успешной загрузке
	msg := tgbotapi.NewMessage(message.Chat.ID, "Файл успешно загружен на сервер Telegram. File ID: "+fileIDFromAgent)
	bot.Send(msg)
}

func downloadFile(url string, filePath string) error {
	// Создание директории для загрузок, если она не существует
	os.MkdirAll("downloads", os.ModePerm)

	// Загрузка файла с URL
	response, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Ошибка получения файла по URL: %v", err)
	}
	defer response.Body.Close()

	// Создание файла на локальной системе
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("Ошибка создания файла: %v", err)
	}
	defer file.Close()

	// Запись данных в файл
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return fmt.Errorf("Ошибка записи файла: %v", err)
	}

	return nil
}

func uploadFileToAgent(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("Ошибка открытия файла: %v", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("Ошибка создания части формы: %v", err)
	}
	_, err = io.Copy(part, file)

	err = writer.Close()
	if err != nil {
		return "", fmt.Errorf("Ошибка закрытия writer: %v", err)
	}

	// Отправка запроса к агенту
	resp, err := http.Post("http://localhost:8080/upload", writer.FormDataContentType(), body)
	if err != nil {
		return "", fmt.Errorf("Ошибка отправки POST запроса: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Ошибка чтения ответа: %v", err)
	}

	return string(respBody), nil
}
