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
	// AgentTokenMail: optional, eingeschränktes Token für die Postfach-Auswertung (nur mit AgentToken).
	AgentTokenMail string

	// GmailAddress und GmailAppPassword: optional, nur gemeinsam; leer = keine Mail-Entwürfe.
	GmailAddress     string
	GmailAppPassword string // ohne Leerzeichen (Google zeigt es in 4er-Gruppen)
}

// Load liest DATABASE_URL (Pflicht), PORT (Standard 8080) und GOTENBERG_URL (optional)
// sowie AGENT_TOKEN und AGENT_TOKEN_MAIL (optional, mind. 32 Zeichen, verschieden) und GMAIL_ADDRESS/GMAIL_APP_PASSWORD (optional, nur gemeinsam).
func Load(getenv func(string) string) (Config, error) {
	c := Config{DatabaseURL: getenv("DATABASE_URL"), Port: getenv("PORT"), GotenbergURL: getenv("GOTENBERG_URL"), AgentToken: getenv("AGENT_TOKEN")}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL ist nicht gesetzt")
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	c.AgentTokenMail = getenv("AGENT_TOKEN_MAIL")
	if err := checkToken("AGENT_TOKEN", c.AgentToken); err != nil {
		return Config{}, err
	}
	if err := checkToken("AGENT_TOKEN_MAIL", c.AgentTokenMail); err != nil {
		return Config{}, err
	}
	if c.AgentTokenMail != "" {
		if c.AgentToken == "" {
			return Config{}, errors.New("AGENT_TOKEN_MAIL braucht ein gesetztes AGENT_TOKEN")
		}
		if c.AgentTokenMail == c.AgentToken {
			return Config{}, errors.New("AGENT_TOKEN_MAIL muss sich von AGENT_TOKEN unterscheiden")
		}
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

// checkToken prüft ein optionales Agent-Token: mind. minAgentTokenLen Zeichen, kein Whitespace.
func checkToken(name, token string) error {
	if token != "" && len(token) < minAgentTokenLen {
		return fmt.Errorf("%s muss mindestens %d Zeichen haben", name, minAgentTokenLen)
	}
	if strings.ContainsFunc(token, unicode.IsSpace) {
		return fmt.Errorf("%s darf keine Leerzeichen oder Zeilenumbrüche enthalten", name)
	}
	return nil
}
