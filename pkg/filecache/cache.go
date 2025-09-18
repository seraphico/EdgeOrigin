package filecache

import (
	"context"
	"io"
	"time"
)

// FileInfo 文件信息
type FileInfo struct {
	Key        string    `json:"key"`         // 缓存键
	Size       int64     `json:"size"`        // 文件大小
	MimeType   string    `json:"mime_type"`   // MIME类型
	CreatedAt  time.Time `json:"created_at"`  // 创建时间
	ExpiresAt  time.Time `json:"expires_at"`  // 过期时间
	AccessCount int64    `json:"access_count"` // 访问次数
	LastAccess time.Time `json:"last_access"`  // 最后访问时间
}

// Cache 文件缓存接口
type Cache interface {
	// Set 存储文件到缓存
	Set(ctx context.Context, key string, data io.Reader, mimeType string, ttl time.Duration) error
	
	// Get 从缓存获取文件
	Get(ctx context.Context, key string) (io.ReadCloser, *FileInfo, error)
	
	// Exists 检查文件是否存在
	Exists(ctx context.Context, key string) (bool, error)
	
	// Delete 删除文件
	Delete(ctx context.Context, key string) error
	
	// List 列出所有缓存文件
	List(ctx context.Context) ([]*FileInfo, error)
	
	// GetInfo 获取文件信息
	GetInfo(ctx context.Context, key string) (*FileInfo, error)
	
	// Cleanup 清理过期文件
	Cleanup(ctx context.Context) error
	
	// Close 关闭缓存
	Close() error
	
	// Stats 获取缓存统计信息
	Stats() (*Stats, error)
}

// Stats 缓存统计信息
type Stats struct {
	TotalFiles    int64   `json:"total_files"`     // 总文件数
	TotalSize     int64   `json:"total_size"`      // 总大小（字节）
	HitRate       float64 `json:"hit_rate"`        // 命中率
	MissRate      float64 `json:"miss_rate"`       // 未命中率
	ExpiredFiles  int64   `json:"expired_files"`   // 过期文件数
	LastCleanup   time.Time `json:"last_cleanup"`   // 最后清理时间
}

// Config 缓存配置
type Config struct {
	DataDir        string        `json:"data_dir"`         // 数据目录
	MaxCacheSize   int64         `json:"max_cache_size"`    // 最大缓存大小（字节）
	DefaultTTL     time.Duration `json:"default_ttl"`       // 默认TTL
	CleanupInterval time.Duration `json:"cleanup_interval"` // 清理间隔
	Compression    bool          `json:"compression"`       // 是否压缩
}
