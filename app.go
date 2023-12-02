package main

import (
	"context"
	"crypto/sha1"
	"embed"
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5"
	"html/template"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed templates/*
var resources embed.FS

var t = template.Must(template.ParseFS(resources, "templates/*"))

var conn *pgx.Conn

// Middleware to track User-Agent
func trackUserAgent(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		slog.Info("New visitor", "IP", r.RemoteAddr, "userAgent", userAgent)

		// Call the next handler in the chain
		next(w, r)
	}
}

// Handle the URL shortening logic
func handleShortenURL(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Parse form data
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Error parsing form", http.StatusInternalServerError)
			return
		}

		// Get the submitted URL from the form
		originalURL := r.Form.Get("url")

		if len(originalURL) < 3 {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// Shorten the URL
		shortURL := shortenURL(originalURL)

		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			fmt.Println("Error extracting IP:", err)
			return
		}

		// Insert to DB
		err = insertShortURL(originalURL, shortURL, ip, r.Header.Get("User-Agent"))
		if err != nil {
			log.Fatalf("Error inserting data into 'short_urls' table: %v\n", err)
		}

		// Render the HTML template with the URL
		data := map[string]string{
			"URL": fmt.Sprintf("https://dontclick.xyz/%s", shortURL),
		}

		t.ExecuteTemplate(w, "result.html", data)
		return
	}

	// Extract the path from the request URL
	path := r.URL.Path

	// Remove the leading slash
	identifier := strings.TrimPrefix(path, "/")

	if len(identifier) == 5 {

		// redirect
		originalURL, err := getOriginalURL(identifier)

		if err != nil {
			slog.Error("Cant get original URL", "err", err.Error())
			http.Error(w, "Something went wrong.", http.StatusInternalServerError)
			return
		}

		slog.Info("Redirect", "from", identifier, "to", originalURL)
		err = increaseHits(identifier)
		if err != nil {
			slog.Error("Cant increase hits", "short", identifier, "err", err.Error())
		}

		// Perform the redirect
		http.Redirect(w, r, originalURL, http.StatusFound)
		return
	}

	// Render the HTML form
	t.ExecuteTemplate(w, "index.html", nil)
}

// Shorten the given URL
func shortenURL(longURL string) string {
	// Generate a unique timestamp
	timestamp := time.Now().UnixNano()

	// Concatenate the long URL and timestamp
	data := fmt.Sprintf("%s%d", longURL, timestamp)

	// Calculate SHA-1 hash
	hash := sha1.Sum([]byte(data))

	// Encode the hash using base64
	encoded := base64.URLEncoding.EncodeToString(hash[:])

	// Remove unnecessary characters
	encoded = strings.ReplaceAll(encoded, "-", "")
	encoded = strings.ReplaceAll(encoded, "_", "")
	encoded = strings.ReplaceAll(encoded, "+", "")
	encoded = strings.ReplaceAll(encoded, "/", "")

	// Take the first 5 characters for the short URL
	shortURL := encoded[:5]

	return shortURL
}

func main() {

	// Define flags
	var DB_URL string

	flag.StringVar(&DB_URL, "db_url", "", "Postgres Database URL")

	flag.Parse()

	// Connect to the database
	var err error
	conn, err = pgx.Connect(context.Background(), DB_URL)
	if err != nil {
		slog.Error("Unable to connect to database: ", "err", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	slog.Info("Connected to DB", "db", DB_URL)

	// Set up HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Register handlers with middleware
	http.HandleFunc("/", trackUserAgent(handleShortenURL))

	// Start the server
	slog.Info("Listening on port", "port", port)
	err = http.ListenAndServe(":"+port, nil)
	if err != nil {
		slog.Error(err.Error())
	}
}

func insertShortURL(originalURL, shortURL, creatorIP, creatorUserAgent string) error {
	// Perform the database insert
	_, err := conn.Exec(context.Background(), "INSERT INTO short_urls(original_url, short_url, creator_ip, creator_user_agent) VALUES($1, $2, $3, $4)",
		originalURL, shortURL, creatorIP, creatorUserAgent)
	if err != nil {
		return fmt.Errorf("unable to insert data into 'short_urls' table: %v", err)
	}

	slog.Info("Data inserted into 'short_urls' table", "original_url", originalURL, "short_url", shortURL)
	return nil
}

func getOriginalURL(shortURL string) (string, error) {
	var originalURL string

	// Perform the database query
	row := conn.QueryRow(context.Background(), "SELECT original_url FROM short_urls WHERE short_url = $1", shortURL)
	err := row.Scan(&originalURL)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Handle case where no rows were found (short URL not in the database)
			return "", fmt.Errorf("short URL not found: %v", err)
		}
		// Handle other errors
		return "", fmt.Errorf("unable to retrieve original URL from 'short_urls' table: %v", err)
	}

	return originalURL, nil
}

func increaseHits(shortURL string) error {
	// Perform the database update
	_, err := conn.Exec(context.Background(), "UPDATE short_urls SET hits = hits + 1 WHERE short_url = $1", shortURL)
	if err != nil {
		return fmt.Errorf("unable to update 'hits' column in 'short_urls' table: %v", err)
	}

	slog.Info("Hits increased for short URL", "short_url", shortURL)
	return nil
}
