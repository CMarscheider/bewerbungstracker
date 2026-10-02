// Package config liest die Konfiguration aus Umgebungsvariablen.
package config

import "errors"

// Config enthält alle Einstellungen des Servers.
type Config struct {
	DatabaseURL string
	Port        string
}

// Load liest DATABASE_URL (Pflicht) und PORT (Standard 8080).
func Load(getenv func(string) string) (Config, error) {
	c := Config{DatabaseURL: getenv("DATABASE_URL"), Port: getenv("PORT")}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL ist nicht gesetzt")
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	return c, nil
}
