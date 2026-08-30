package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Runtime returns the embedded Bee worker configuration. Persistent files are
// always placed under the application-data directory supplied by Bee.
func Runtime(dataDir string) Config {
	database := DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(dataDir, "data", "xianlv.db"), MaxOpenConnections: 4, MaxIdleConnections: 2}
	if cloudDSN := strings.TrimSpace(os.Getenv("XIANLV_CLOUD_DATABASE_DSN")); cloudDSN != "" {
		database = DatabaseConfig{Driver: "postgres", DSN: cloudDSN, MaxOpenConnections: 10, MaxIdleConnections: 5}
	}
	return Config{
		Server:   ServerConfig{Address: "127.0.0.1:8088", ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, AllowedOrigins: []string{"*"}},
		Database: database,
		Redis:    RedisConfig{Enabled: false},
		Backup:   BackupConfig{Directory: filepath.Join(dataDir, "backups"), KeepDays: 30},
	}
}

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Admin     AdminConfig
	Backup    BackupConfig
	Scheduler SchedulerConfig
}

type ServerConfig struct {
	Address        string
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	AllowedOrigins []string      `mapstructure:"allowed_origins"`
}

type DatabaseConfig struct {
	Driver             string
	DSN                string
	MaxOpenConnections int `mapstructure:"max_open_connections"`
	MaxIdleConnections int `mapstructure:"max_idle_connections"`
}

type RedisConfig struct {
	Enabled  bool
	Address  string
	Password string
	Database int
}

type AdminConfig struct {
	BootstrapToken     string `mapstructure:"bootstrap_token"`
	RateLimitPerMinute int    `mapstructure:"rate_limit_per_minute"`
}

type BackupConfig struct {
	Directory string
	KeepDays  int `mapstructure:"keep_days"`
}

type SchedulerConfig struct {
	DailyReset     string `mapstructure:"daily_reset"`
	RankingRefresh string `mapstructure:"ranking_refresh"`
	Backup         string
}

func Load(path string) (Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("XIANLV")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetDefault("server.address", "127.0.0.1:8088")
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", "data/xianlv.db")
	v.SetDefault("database.max_open_connections", 20)
	v.SetDefault("database.max_idle_connections", 5)
	v.SetDefault("admin.rate_limit_per_minute", 180)
	v.SetDefault("backup.directory", "backups")
	v.SetDefault("backup.keep_days", 30)
	v.SetDefault("scheduler.daily_reset", "0 0 0 * * *")
	v.SetDefault("scheduler.ranking_refresh", "0 */10 * * * *")
	v.SetDefault("scheduler.backup", "0 30 3 * * *")
	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}
