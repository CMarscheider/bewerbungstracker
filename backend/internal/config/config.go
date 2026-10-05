// Package config liest die Konfiguration aus Umgebungsvariablen.
package config

import (
	"errors"
	"fmt"
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
}

// Load liest DATABASE_URL (Pflicht), PORT (Standard 8080) und GOTENBERG_URL (optional)
// sowie AGENT_TOKEN (optional, mind. 32 Zeichen).
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
	return c, nil
}
