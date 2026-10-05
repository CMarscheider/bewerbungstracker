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

func TestLoadGmail(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": "postgres://x", "GMAIL_ADDRESS": " erika@gmail.com ", "GMAIL_APP_PASSWORD": "abcd efgh ijkl mnop"}))
	if err != nil || c.GmailAddress != "erika@gmail.com" || c.GmailAppPassword != "abcdefghijklmnop" {
		t.Fatalf("Gmail = %q/%q, err %v", c.GmailAddress, c.GmailAppPassword, err)
	}
	c, err = Load(env(map[string]string{"DATABASE_URL": "postgres://x"}))
	if err != nil || c.GmailAddress != "" || c.GmailAppPassword != "" {
		t.Fatalf("ohne Gmail: %+v, %v", c, err)
	}
	for _, m := range []map[string]string{
		{"DATABASE_URL": "postgres://x", "GMAIL_ADDRESS": "erika@gmail.com"},
		{"DATABASE_URL": "postgres://x", "GMAIL_APP_PASSWORD": "abcd efgh ijkl mnop"},
	} {
		if _, err := Load(env(m)); err == nil || !strings.Contains(err.Error(), "gehören zusammen") {
			t.Errorf("%v: erwartet Fehler, bekommen %v", m, err)
		}
	}
	if _, err := Load(env(map[string]string{"DATABASE_URL": "postgres://x", "GMAIL_ADDRESS": "Erika <erika@gmail.com>", "GMAIL_APP_PASSWORD": "x"})); err == nil {
		t.Error("GMAIL_ADDRESS mit Namen muss abgelehnt werden")
	}
}

func TestLoadRejectsWhitespaceInAgentToken(t *testing.T) {
	a, b := strings.Repeat("a", 32), strings.Repeat("b", 16)
	for _, tok := range []string{a + "\r", a + "\n", b + " " + b} {
		if _, err := Load(env(map[string]string{"DATABASE_URL": "postgres://x", "AGENT_TOKEN": tok})); err == nil {
			t.Errorf("AGENT_TOKEN %q mit Whitespace muss abgelehnt werden", tok)
		}
	}
}
