package config

import (
	"strings"
	"testing"
)

func loadLogConfig(t *testing.T, body string) (*Config, error) {
	t.Helper()
	return Load(writeTOML(t, `
default = "personal"

[browsers.personal]
browser = "chrome"
`+body))
}

func TestLogTableParsesAllFields(t *testing.T) {
	cfg, err := loadLogConfig(t, `
[log]
enabled = true
path = "/var/log/brouter/routing.log"
host = true
max_bytes = 4096
max_files = 5
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Log == nil || !cfg.Log.Enabled || !cfg.Log.Host {
		t.Fatalf("log flags not parsed: %+v", cfg.Log)
	}
	if cfg.Log.Path != "/var/log/brouter/routing.log" || cfg.Log.MaxBytes != 4096 || cfg.Log.MaxFiles != 5 {
		t.Fatalf("log values not parsed: %+v", cfg.Log)
	}
}

func TestAbsentLogTableMeansDisabled(t *testing.T) {
	cfg, err := loadLogConfig(t, "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Log != nil {
		t.Fatalf("absent [log] must stay nil, got %+v", cfg.Log)
	}
}

func TestDisabledLogTableIgnoresBounds(t *testing.T) {
	cfg, err := loadLogConfig(t, `
[log]
enabled = false
max_bytes = 1
`)
	if err != nil {
		t.Fatalf("disabled table must not fail bounds: %v", err)
	}
	if cfg.Log == nil || cfg.Log.Enabled {
		t.Fatalf("expected present-but-disabled log config, got %+v", cfg.Log)
	}
}

func TestLogPathMustBeAbsoluteOrHome(t *testing.T) {
	for _, path := range []string{"relative/log", "./log", ""} {
		if path == "" {
			continue
		}
		_, err := loadLogConfig(t, `
[log]
enabled = true
path = "`+path+`"
`)
		if err == nil || !strings.Contains(err.Error(), "log: path must be absolute") {
			t.Fatalf("path %q: expected visible rejection, got %v", path, err)
		}
	}
	for _, path := range []string{"/var/log/routing.log", "~/logs/routing.log"} {
		if _, err := loadLogConfig(t, `
[log]
enabled = true
path = "`+path+`"
`); err != nil {
			t.Fatalf("path %q must be accepted: %v", path, err)
		}
	}
}

func TestLogBoundsAreValidated(t *testing.T) {
	_, err := loadLogConfig(t, `
[log]
enabled = true
max_bytes = 10
`)
	if err == nil || !strings.Contains(err.Error(), "max_bytes") {
		t.Fatalf("expected max_bytes rejection, got %v", err)
	}
	_, err = loadLogConfig(t, `
[log]
enabled = true
max_files = 500
`)
	if err == nil || !strings.Contains(err.Error(), "max_files") {
		t.Fatalf("expected max_files rejection, got %v", err)
	}
}

func TestLogUnknownKeysAreRejected(t *testing.T) {
	_, err := loadLogConfig(t, `
[log]
enabled = true
verbose = true
`)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown-field rejection, got %v", err)
	}
}
