package config

import "testing"

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
