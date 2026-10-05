package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	if _, err := Load(env(nil)); err == nil {
		t.Fatal("erwartet Fehler ohne DATABASE_URL")
	}
}

func TestLoadDefaultsPort(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": "postgres://x"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "8080" || c.DatabaseURL != "postgres://x" {
		t.Errorf("Config = %+v", c)
	}
}

func TestLoadExplicitPort(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": "postgres://x", "PORT": "9000"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "9000" {
		t.Errorf("Port = %s", c.Port)
	}
}

func TestLoadReadsGotenbergURL(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": "postgres://x", "GOTENBERG_URL": "http://gotenberg:3000"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.GotenbergURL != "http://gotenberg:3000" {
		t.Errorf("GotenbergURL = %q", c.GotenbergURL)
	}
}

func TestLoadAgentToken(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": "postgres://x", "AGENT_TOKEN": strings.Repeat("a", 32)}))
	if err != nil || c.AgentToken != strings.Repeat("a", 32) {
		t.Fatalf("AgentToken = %q, err %v", c.AgentToken, err)
	}
	if _, err := Load(env(map[string]string{"DATABASE_URL": "postgres://x", "AGENT_TOKEN": "zu-kurz"})); err == nil {
		t.Fatal("zu kurzes AGENT_TOKEN muss abgelehnt werden")
	}
	c, err = Load(env(map[string]string{"DATABASE_URL": "postgres://x"}))
	if err != nil || c.AgentToken != "" {
		t.Fatalf("ohne AGENT_TOKEN: %q, %v", c.AgentToken, err)
	}
}
