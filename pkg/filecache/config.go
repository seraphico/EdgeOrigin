package filecache

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		DataDir:         "./cache",
		MaxCacheSize:    1024 * 1024 * 1024, // 1GB
		DefaultTTL:      24 * time.Hour,
		CleanupInterval: time.Hour,
		Compression:     true,
	}
}

// LoadConfigFromFile 从文件加载配置
func LoadConfigFromFile(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// SaveConfigToFile 保存配置到文件
func SaveConfigToFile(config *Config, filename string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// ValidateConfig 验证配置
func ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if config.DataDir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}

	if config.MaxCacheSize <= 0 {
		return fmt.Errorf("max cache size must be positive")
	}

	if config.DefaultTTL <= 0 {
		return fmt.Errorf("default TTL must be positive")
	}

	if config.CleanupInterval <= 0 {
		return fmt.Errorf("cleanup interval must be positive")
	}

	return nil
}

// NewCacheWithConfig 使用配置创建缓存
func NewCacheWithConfig(config *Config) (Cache, error) {
	if err := ValidateConfig(config); err != nil {
		return nil, err
	}

	return NewBadgerCache(config)
}
