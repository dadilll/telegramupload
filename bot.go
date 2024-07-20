package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

var fileID string

func main() {
	// Define constants
	botToken := "" // Your bot token

	// Create a new bot instance
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// Set up update configuration
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// Start receiving updates
	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatalf("Failed to get updates: %v", err)
	}

	// Start HTTP server to receive file_id from the agent
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		var data map[string]string
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(body, &data); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		fileID = data["file_id"]
		fmt.Fprintf(w, "file_id received: %s", fileID)
		log.Printf("Received file_id: %s", fileID)
	})

	go func() {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// Process each update
	for update := range updates {
		if update.Message != nil {
			log.Printf("Received message: %s", update.Message.Text)

			// Check if the message is a command
			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "monkey":
					// Handle /monkey command
					if fileID == "" {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "file_id not available yet.")
						bot.Send(msg)
					} else {
						sendVideoByFileID(bot, update.Message.Chat.ID, fileID)
					}
				default:
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unknown command")
					bot.Send(msg)
				}
			}
		}
	}
}

// sendVideoByFileID sends a video using file_id to the specified chat
func sendVideoByFileID(bot *tgbotapi.BotAPI, chatID int64, fileID string) {
	chatIDStr := strconv.FormatInt(chatID, 10) // Convert chatID to string for logging
	log.Printf("Sending video to chat ID: %s with file_id: %s", chatIDStr, fileID)
	msg := tgbotapi.NewVideoShare(chatID, fileID)
	msg.Caption = "Here's a monkey video for you!"
	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Failed to send video: %v", err)
	} else {
		log.Printf("Video sent successfully to chat ID: %s", chatIDStr)
	}
}
