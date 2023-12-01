package main

import (
	"crypto/sha1"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed templates/*
var resources embed.FS

var t = template.Must(template.ParseFS(resources, "templates/*"))

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Parse form data
			err := r.ParseForm()
			if err != nil {
				http.Error(w, "Error parsing form", http.StatusInternalServerError)
				return
			}

			// Get the submitted URL from the form
			url := r.Form.Get("url")

			// shorten the URL
			shortURL := shortenURL(url)

			// Render the HTML template with the URL
			data := map[string]string{
				"URL": shortURL,
			}

			t.ExecuteTemplate(w, "result.html", data)
			return
		}

		// Render the HTML form
		data := map[string]string{
			"Region": os.Getenv("FLY_REGION"),
		}

		t.ExecuteTemplate(w, "index.html", data)
	})

	log.Println("Listening on port", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func shortenURL(longURL string) string {
	// Generate a unique timestamp
	timestamp := time.Now().UnixNano()

	// Concatenate the long URL and timestamp
	data := fmt.Sprintf("%s%d", longURL, timestamp)

	// Calculate SHA-1 hash
	hash := sha1.Sum([]byte(data))

	// Encode the hash using base64
	encoded := base64.URLEncoding.EncodeToString(hash[:])

	encoded = strings.ReplaceAll(encoded, "-", "")
	encoded = strings.ReplaceAll(encoded, "_", "")
	encoded = strings.ReplaceAll(encoded, "+", "")
	encoded = strings.ReplaceAll(encoded, "/", "")

	// Take the first 5 characters for the short URL
	shortURL := encoded[:5]

	return fmt.Sprintf("https://dontclick.xyz/%s", shortURL)
}
