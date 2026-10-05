// Package config liest die Konfiguration aus Umgebungsvariablen.
package config

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode"
)

// minAgentTokenLen schützt vor schwachen Tokens; z. B. `openssl rand -hex 32` erzeugt 64 Zeichen.
const minAgentTokenLen = 32

// Config enthält alle Einstellungen des Servers.
type Config struct {
	DatabaseURL  string
	Port         string
	GotenbergURL string // optional; leer = keine PDF-Erzeugung
	AgentToken   string // optional; leer = keine Agent-API

	// GmailAddress und GmailAppPassword: optional, nur gemeinsam; leer = keine Mail-Entwürfe.
	GmailAddress     string
	GmailAppPassword string // ohne Leerzeichen (Google zeigt es in 4er-Gruppen)
}

// Load liest DATABASE_URL (Pflicht), PORT (Standard 8080) und GOTENBERG_URL (optional)
// sowie AGENT_TOKEN (optional, mind. 32 Zeichen) und GMAIL_ADDRESS/GMAIL_APP_PASSWORD (optional, nur gemeinsam).
func Load(getenv func(string) string) (Config, error) {
	c := Config{DatabaseURL: getenv("DATABASE_URL"), Port: getenv("PORT"), GotenbergURL: getenv("GOTENBERG_URL"), AgentToken: getenv("AGENT_TOKEN")}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL ist nicht gesetzt")
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	if c.AgentToken != "" && len(c.AgentToken) < minAgentTokenLen {
		return Config{}, fmt.Errorf("AGENT_TOKEN muss mindestens %d Zeichen haben", minAgentTokenLen)
	}
	if strings.ContainsFunc(c.AgentToken, unicode.IsSpace) {
		return Config{}, errors.New("AGENT_TOKEN darf keine Leerzeichen oder Zeilenumbrüche enthalten")
	}
	c.GmailAddress = strings.TrimSpace(getenv("GMAIL_ADDRESS"))
	c.GmailAppPassword = strings.Join(strings.Fields(getenv("GMAIL_APP_PASSWORD")), "")
	if (c.GmailAddress == "") != (c.GmailAppPassword == "") {
		return Config{}, errors.New("GMAIL_ADDRESS und GMAIL_APP_PASSWORD gehören zusammen")
	}
	if c.GmailAddress != "" {
		if a, err := mail.ParseAddress(c.GmailAddress); err != nil || a.Name != "" || a.Address != c.GmailAddress {
			return Config{}, errors.New("GMAIL_ADDRESS muss eine reine Mail-Adresse sein, z. B. name@gmail.com")
		}
	}
	return c, nil
}
