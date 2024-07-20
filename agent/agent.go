package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"

	"github.com.gotd/td/telegram/auth"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
)

// codeAuthenticator implements the auth.CodeAuthenticator interface
type codeAuthenticator struct{}

// Code requests the code from the user
func (c *codeAuthenticator) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Введите код подтверждения: ")
	code, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("не удалось считать код: %w", err)
	}
	return strings.TrimSpace(code), nil
}

// UploadHandler handles video upload requests
func UploadHandler(client *telegram.Client, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, handler, err := r.FormFile("video")
		if err != nil {
			http.Error(w, "Unable to retrieve file from form", http.StatusBadRequest)
			log.Printf("Unable to retrieve file from form: %v", err)
			return
		}
		defer file.Close()

		log.Printf("Uploaded file: %s", handler.Filename)

		// Create a temporary file to save the uploaded video
		tempFile, err := os.CreateTemp("", "upload-*.mp4")
		if err != nil {
			http.Error(w, "Unable to create temporary file", http.StatusInternalServerError)
			log.Printf("Unable to create temporary file: %v", err)
			return
		}
		defer os.Remove(tempFile.Name()) // Clean up temp file afterwards

		// Write the uploaded file to the temporary file
		if _, err := io.Copy(tempFile, file); err != nil {
			http.Error(w, "Unable to save file", http.StatusInternalServerError)
			log.Printf("Unable to save file: %v", err)
			return
		}

		log.Println("File saved to temp directory")

		// Upload the video file using Telegram API
		upl := uploader.NewUploader(client.API())
		video, err := upl.FromPath(ctx, tempFile.Name())
		if err != nil {
			http.Error(w, "Failed to upload video", http.StatusInternalServerError)
			log.Printf("Failed to upload video: %v", err)
			return
		}

		log.Println("Video uploaded to Telegram")

		// Send the video to the bot to get file_id
		botUsername := "AdventureBot" // Replace with your bot's username
		botUser, err := client.API().ContactsResolveUsername(ctx, botUsername)
		if err != nil {
			http.Error(w, "Failed to resolve bot username", http.StatusInternalServerError)
			log.Printf("Failed to resolve bot username: %v", err)
			return
		}

		peer := &tg.InputPeerUser{UserID: botUser.Users[0].(*tg.User).ID}

		msg, err := client.API().MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
			Peer: peer,
			Media: &tg.InputMediaUploadedDocument{
				File:     video,
				MimeType: "video/mp4",
				Attributes: []tg.DocumentAttributeClass{
					&tg.DocumentAttributeVideo{
						RoundMessage:      false,
						SupportsStreaming: true,
					},
				},
				ForceFile: false,
			},
			Message:  "Получите видео!",
			RandomID: rand.Int63(),
		})
		if err != nil {
			http.Error(w, "Failed to send video to bot", http.StatusInternalServerError)
			log.Printf("Failed to send video to bot: %v", err)
			return
		}

		log.Println("Video sent to bot")

		// Extract file_id from the response
		var fileID int64
		updates, ok := msg.(*tg.Updates)
		if !ok {
			http.Error(w, "Invalid message format", http.StatusInternalServerError)
			log.Printf("Invalid message format: %v", msg)
			return
		}
		for _, update := range updates.Updates {
			if upd, ok := update.(*tg.UpdateNewMessage); ok {
				// Ensure Message is of the correct type
				if msg, ok := upd.Message.(*tg.Message); ok {
					if media, ok := msg.Media.(*tg.MessageMediaDocument); ok {
						if doc, ok := media.Document.(*tg.Document); ok {
							fileID = doc.ID
							break
						}
					}
				}
			}
		}

		log.Printf("File ID: %d", fileID)

		// Send file_id to bot server
		if err := sendFileIDToBotServer(fileID); err != nil {
			http.Error(w, "Failed to send file_id to bot server", http.StatusInternalServerError)
			log.Printf("Failed to send file_id to bot server: %v", err)
			return
		}

		// Respond with the file_id
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int64{"file_id": fileID})
	}
}

func main() {
	// Replace with your api_id and api_hash
	apiID :=                              // Replace with your api_id
	apiHash := "" // Replace with your api_hash
	telegramPhone := ""               // Replace with your phone number

	client := telegram.NewClient(apiID, apiHash, telegram.Options{})
	ctx := context.Background()

	// Create a new session
	err := client.Run(ctx, func(ctx context.Context) error {
		// Authentication process
		if err := client.Auth().IfNecessary(ctx, auth.NewFlow(
			auth.CodeOnly(telegramPhone, &codeAuthenticator{}), // added code handler
			auth.SendCodeOptions{},
		)); err != nil {
			return fmt.Errorf("ошибка аутентификации: %w", err)
		}

		// Set up HTTP server to receive video files
		http.HandleFunc("/upload", UploadHandler(client, ctx))
		fmt.Println("Listening on :8081 for video uploads...")
		if err := http.ListenAndServe(":8081", nil); err != nil {
			return fmt.Errorf("failed to start HTTP server: %w", err)
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Произошла ошибка: %v", err)
	}
}

// sendFileIDToBotServer sends the file_id to the bot's server
func sendFileIDToBotServer(fileID int64) error {
	botServerURL := "http://localhost:8080/upload" // URL бота-сервера

	data := map[string]int64{"file_id": fileID}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("не удалось маршалировать данные: %w", err)
	}

	resp, err := http.Post(botServerURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("ошибка HTTP запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ошибка HTTP ответа: статус %s", resp.Status)
	}

	return nil
}
