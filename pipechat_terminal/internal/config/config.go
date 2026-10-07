// Package config loads runtime settings from the environment and an optional
// .env file in the directory the program starts from. Real environment
// variables win over the file.
package config

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// Server.
	Host        string // address to listen on; empty means all addresses
	Port        string
	DatabaseURL string // PostgreSQL connection URL
	// EscrowKey (CHAT_ESCROW_KEY, 64 hex characters) lets the server hold website groups' keys so
	// invitees get in at once. Empty turns that off: keys then pass from member to member.
	EscrowKey []byte
	// WebsiteURL (CHAT_WEBSITE_URL, or the site's APP_URL) is where people register; the terminal shows it on
	// its login screen. Empty means the website is at the same address as this server.
	WebsiteURL string
	// StorageDir is the website's storage/app/private folder, where uploaded music lives.
	StorageDir string
	TLSCert    string
	TLSKey     string

	// Encrypted files shared in chats are kept in FilesDir until they are FileDays old.
	FilesDir       string
	MaxUploadBytes int64
	FileDays       int

	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
}

// Load reads the optional .env file, then builds the Config.
func Load() (*Config, error) {
	loadDotEnv(".env")
	loadDotEnv("../.env") // the website's file, when the server runs from pipechat_terminal/

	maxUploadMB, err := strconv.Atoi(get("CHAT_MAX_UPLOAD_MB", "500"))
	if err != nil || maxUploadMB < 1 {
		return nil, fmt.Errorf("CHAT_MAX_UPLOAD_MB must be a whole number of megabytes, 1 or more")
	}
	fileDays, err := strconv.Atoi(get("CHAT_FILE_DAYS", "7"))
	if err != nil || fileDays < 1 {
		return nil, fmt.Errorf("CHAT_FILE_DAYS must be a whole number of days, 1 or more")
	}

	escrow, err := hex.DecodeString(get("CHAT_ESCROW_KEY", ""))
	if err != nil || (len(escrow) != 0 && len(escrow) != 32) {
		return nil, fmt.Errorf("CHAT_ESCROW_KEY must be 64 hex characters (make one with: openssl rand -hex 32)")
	}

	settings := &Config{
		EscrowKey:      escrow,
		FilesDir:       get("CHAT_FILES_DIR", "data/files"),
		MaxUploadBytes: int64(maxUploadMB) << 20,
		FileDays:       fileDays,
		Host:           get("CHAT_HOST", ""),
		Port:           get("CHAT_PORT", "7777"),
		DatabaseURL:    databaseURL(),
		StorageDir:     get("CHAT_STORAGE_DIR", "../storage/app/private"),
		WebsiteURL:     websiteURL(get("CHAT_WEBSITE_URL", get("APP_URL", ""))),
		TLSCert:        get("CHAT_TLS_CERT", ""),
		TLSKey:         get("CHAT_TLS_KEY", ""),
		SMTPHost:       get("SMTP_HOST", websiteMail("MAIL_HOST")),
		SMTPPort:       get("SMTP_PORT", get("MAIL_PORT", "587")),
		SMTPUsername:   get("SMTP_USERNAME", websiteMail("MAIL_USERNAME")),
		SMTPPassword:   get("SMTP_PASSWORD", websiteMail("MAIL_PASSWORD")),
		SMTPFrom:       get("SMTP_FROM", websiteMail("MAIL_FROM_ADDRESS")),
	}
	if (settings.TLSCert == "") != (settings.TLSKey == "") {
		return nil, fmt.Errorf("CHAT_TLS_CERT and CHAT_TLS_KEY must be set together")
	}
	return settings, nil
}

// databaseURL is CHAT_DATABASE_URL, or else built from the DB_* settings the
// Laravel website already uses, so both talk to the same database.
func databaseURL() string {
	if explicit := get("CHAT_DATABASE_URL", ""); explicit != "" {
		return explicit
	}
	address := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(get("DB_USERNAME", "postgres"), get("DB_PASSWORD", "")),
		Host:   net.JoinHostPort(get("DB_HOST", "127.0.0.1"), get("DB_PORT", "5432")),
		Path:   get("DB_DATABASE", "pipechat_website"),
	}
	return address.String()
}

func get(key string, fallback string) string {
	if value, found := os.LookupEnv(key); found && value != "" {
		return value
	}
	return fallback
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' || value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

// websiteURL tidies an address typed without its scheme: a plain domain means https://, while localhost, an IP
// address or one with a port means http://, the way a site run by hand answers.
func websiteURL(raw string) string {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" || strings.Contains(raw, "://") {
		return raw
	}
	parsed, err := url.Parse("//" + raw)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	if parsed.Port() == "" && strings.Contains(parsed.Hostname(), ".") && net.ParseIP(parsed.Hostname()) == nil {
		return "https://" + raw
	}
	return "http://" + raw
}

// websiteMail reads one of the website's mail settings, but only when the website really sends by SMTP: its
// "log" mailer writes mails to a file, and its default host and sender are placeholders, not a mail server.
func websiteMail(key string) string {
	if os.Getenv("MAIL_MAILER") != "smtp" {
		return ""
	}
	value := os.Getenv(key)
	if value == "null" {
		return ""
	}
	return value
}
