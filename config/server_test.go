package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.PublicAddr != "0.0.0.0:8080" || cfg.DeviceAddr != "0.0.0.0:8081" {
		t.Fatalf("unexpected default addresses: %+v", cfg)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("PUBLIC_ADDR", ":9000")
	t.Setenv("DEVICE_ADDR", ":9001")
	t.Setenv("LOG_LEVEL", "debug")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.PublicAddr != ":9000" || cfg.DeviceAddr != ":9001" || cfg.LogLevel != "debug" {
		t.Fatalf("env not applied: %+v", cfg)
	}
}

func TestValidate(t *testing.T) {
	valid := ServerConfig{PublicAddr: ":8080", DeviceAddr: ":8081", LogLevel: "info", Environment: "development"}
	tests := []struct {
		name    string
		mutate  func(*ServerConfig)
		wantErr bool
	}{
		{"valid development", func(*ServerConfig) {}, false},
		{"same addresses", func(c *ServerConfig) { c.DeviceAddr = c.PublicAddr }, true},
		{"bad log level", func(c *ServerConfig) { c.LogLevel = "verbose" }, true},
		{"production without database", func(c *ServerConfig) { c.Environment = EnvProduction }, true},
		{"production with database", func(c *ServerConfig) {
			c.Environment = EnvProduction
			c.DatabaseURL = "postgres://x"
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)
			if err := cfg.Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
