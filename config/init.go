package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Конечная структура конфигурации приложения.
// Расширяем по мере роста проекта.
type Config struct {
	Server struct {
		Address  string `mapstructure:"address"`   // 0.0.0.0
		HTTPPort string `mapstructure:"http_port"` // 8080
	} `mapstructure:"server"`

	OpenWISP struct {
		SharedSecret string `mapstructure:"shared_secret"` // секрет для агента
		Controller   struct {
			PKI struct {
				CAName  string `mapstructure:"ca_name"`  // "OpenWISP CA"
				CertTTL string `mapstructure:"cert_ttl"` // "8760h" (год)
			} `mapstructure:"pki"`
			MgmtVPN struct {
				Mode      string `mapstructure:"mode"` // "wireguard"|"openvpn"|"zerotier"|"none"
				WireGuard struct {
					Endpoint        string   `mapstructure:"endpoint"`          // "vpn.example.com:51820"
					ServerPublicKey string   `mapstructure:"server_public_key"` // публичный ключ сервера
					AddressPoolCIDR string   `mapstructure:"address_pool_cidr"` // "10.10.0.0/24"
					AllowedIPs      []string `mapstructure:"allowed_ips"`       // ["10.10.0.0/24"]
					Keepalive       int      `mapstructure:"keepalive"`         // 25 (сек)
				} `mapstructure:"wireguard"`
				OpenVPN struct {
					Remote string `mapstructure:"remote"` // "vpn.example.com"
					Port   int    `mapstructure:"port"`   // 1194
					Proto  string `mapstructure:"proto"`  // "udp"
					Cipher string `mapstructure:"cipher"` // "AES-256-GCM"
					Auth   string `mapstructure:"auth"`   // "SHA256"
				} `mapstructure:"openvpn"`
				ZeroTier struct {
					NetworkID string `mapstructure:"networkid"`
					Token     string // опционально
				} `mapstructure:"zerotier"`
			} `mapstructure:"mgmtVPN"`
		} `mapstructure:"controller"`
	} `mapstructure:"openwisp"`

	Logging struct {
		Level  string `mapstructure:"level"`  // trace|debug|info|warning|error|fatal
		Format string `mapstructure:"format"` // text|json
		File   string `mapstructure:"file"`   // путь/префикс файла, пусто — только stdout
	} `mapstructure:"logs"`

	Database struct {
		Driver string `mapstructure:"driver"` // "postgres" | "sqlite" | "" (in-memory)
		DSN    string `mapstructure:"dsn"`    // пример: postgres://user:pass@host:5432/dbname?sslmode=disable
	} `mapstructure:"database"`
}

// Load читает конфиг из env/файла с дефолтами.
func Load() (*Config, error) {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Дефолты (минимальный рабочий набор)
	viper.SetDefault("server.address", "0.0.0.0")
	viper.SetDefault("server.http_port", "8080")
	viper.SetDefault("openwisp.shared_secret", "CHANGE_ME")

	// Логи — дефолты
	viper.SetDefault("logs.level", "info")
	viper.SetDefault("logs.format", "text")
	viper.SetDefault("logs.file", "")

	// DB: по умолчанию — in-memory (пустой driver)
	viper.SetDefault("database.driver", "")
	viper.SetDefault("database.dsn", "")

	// Источник файла
	if cfgFile := os.Getenv("CONFIG_FILE"); cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			viper.AddConfigPath(filepath.Join(xdg, "openwisp-go"))
		}
		viper.AddConfigPath("/etc/openwisp-go")
	}

	// Чтение файла (опционально)
	if err := viper.ReadInConfig(); err != nil {
		var nf viper.ConfigFileNotFoundError
		if !errors.As(err, &nf) {
			return nil, fmt.Errorf("config read error: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config unmarshal error: %w", err)
	}
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}
	return cfg
}

func validate(c *Config) error {
	if strings.TrimSpace(c.OpenWISP.SharedSecret) == "" || c.OpenWISP.SharedSecret == "CHANGE_ME" {
		return errors.New("openwisp.shared_secret must be set (not empty and not CHANGE_ME)")
	}
	if strings.TrimSpace(c.Server.Address) == "" {
		return errors.New("server.address must not be empty")
	}
	if strings.TrimSpace(c.Server.HTTPPort) == "" {
		return errors.New("server.http_port must not be empty")
	}
	return nil
}
