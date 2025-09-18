package filecache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
)

const (
	// 键前缀
	fileDataPrefix = "file:"
	fileInfoPrefix = "info:"
	statsKey       = "stats"
)

// badgerCache Badger文件缓存实现
type badgerCache struct {
	db     *badger.DB
	config *Config
	stats  *Stats
	mu     sync.RWMutex
}

// NewBadgerCache 创建新的Badger文件缓存
func NewBadgerCache(config *Config) (Cache, error) {
	if config == nil {
		config = &Config{
			DataDir:         "./cache",
			MaxCacheSize:    1024 * 1024 * 1024, // 1GB
			DefaultTTL:      24 * time.Hour,
			CleanupInterval: time.Hour,
			Compression:     true,
		}
	}

	// 确保数据目录存在
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// 配置Badger选项
	opts := badger.DefaultOptions(filepath.Join(config.DataDir, "badger"))
	opts.Logger = nil // 禁用日志
	if config.Compression {
		opts.Compression = options.ZSTD
	} else {
		opts.Compression = options.None
	}
	opts.ValueLogFileSize = 64 << 20 // 64MB

	// 打开数据库
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger database: %w", err)
	}

	cache := &badgerCache{
		db:     db,
		config: config,
		stats:  &Stats{},
	}

	// 加载统计信息
	if err := cache.loadStats(); err != nil {
		// 如果加载失败，使用默认统计信息
		cache.stats = &Stats{}
	}

	// 启动清理协程
	go cache.startCleanupRoutine()

	return cache, nil
}

// Set 存储文件到缓存
func (c *badgerCache) Set(ctx context.Context, key string, data io.Reader, mimeType string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.config.DefaultTTL
	}

	// 读取数据到内存
	dataBytes, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	// 检查缓存大小限制
	if int64(len(dataBytes)) > c.config.MaxCacheSize {
		return fmt.Errorf("file size %d exceeds max cache size %d", len(dataBytes), c.config.MaxCacheSize)
	}

	now := time.Now()
	expiresAt := now.Add(ttl)

	// 创建文件信息
	fileInfo := &FileInfo{
		Key:         key,
		Size:        int64(len(dataBytes)),
		MimeType:    mimeType,
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
		AccessCount: 0,
		LastAccess:  now,
	}

	// 序列化文件信息
	infoBytes, err := json.Marshal(fileInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal file info: %w", err)
	}

	// 存储到Badger
	err = c.db.Update(func(txn *badger.Txn) error {
		// 存储文件数据
		if err := txn.Set([]byte(fileDataPrefix+key), dataBytes); err != nil {
			return err
		}
		// 存储文件信息
		return txn.Set([]byte(fileInfoPrefix+key), infoBytes)
	})

	if err != nil {
		return fmt.Errorf("failed to store file: %w", err)
	}

	// 更新统计信息
	c.updateStatsAfterSet(int64(len(dataBytes)))

	return nil
}

// Get 从缓存获取文件
func (c *badgerCache) Get(ctx context.Context, key string) (io.ReadCloser, *FileInfo, error) {
	var fileInfo *FileInfo
	var data []byte

	err := c.db.View(func(txn *badger.Txn) error {
		// 获取文件信息
		infoItem, err := txn.Get([]byte(fileInfoPrefix + key))
		if err != nil {
			return err
		}

		err = infoItem.Value(func(val []byte) error {
			fileInfo = &FileInfo{}
			return json.Unmarshal(val, fileInfo)
		})
		if err != nil {
			return err
		}

		// 检查是否过期
		if time.Now().After(fileInfo.ExpiresAt) {
			return fmt.Errorf("file expired")
		}

		// 获取文件数据
		dataItem, err := txn.Get([]byte(fileDataPrefix + key))
		if err != nil {
			return err
		}

		return dataItem.Value(func(val []byte) error {
			data = make([]byte, len(val))
			copy(data, val)
			return nil
		})
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			c.updateStatsAfterMiss()
			return nil, nil, fmt.Errorf("file not found")
		}
		return nil, nil, err
	}

	// 更新访问统计
	c.updateStatsAfterHit()
	c.updateFileAccess(key, fileInfo)

	return &readCloser{data: data}, fileInfo, nil
}

// Exists 检查文件是否存在
func (c *badgerCache) Exists(ctx context.Context, key string) (bool, error) {
	exists := false
	err := c.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(fileInfoPrefix + key))
		if err == badger.ErrKeyNotFound {
			exists = false
			return nil
		}
		if err != nil {
			return err
		}
		exists = true
		return nil
	})
	return exists, err
}

// Delete 删除文件
func (c *badgerCache) Delete(ctx context.Context, key string) error {
	var fileInfo *FileInfo

	// 先获取文件信息以更新统计
	err := c.db.View(func(txn *badger.Txn) error {
		infoItem, err := txn.Get([]byte(fileInfoPrefix + key))
		if err != nil {
			return err
		}
		return infoItem.Value(func(val []byte) error {
			fileInfo = &FileInfo{}
			return json.Unmarshal(val, fileInfo)
		})
	})

	if err != nil && err != badger.ErrKeyNotFound {
		return err
	}

	// 删除文件
	err = c.db.Update(func(txn *badger.Txn) error {
		if err := txn.Delete([]byte(fileDataPrefix + key)); err != nil {
			return err
		}
		return txn.Delete([]byte(fileInfoPrefix + key))
	})

	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// 更新统计信息
	if fileInfo != nil {
		c.updateStatsAfterDelete(fileInfo.Size)
	}

	return nil
}

// List 列出所有缓存文件
func (c *badgerCache) List(ctx context.Context) ([]*FileInfo, error) {
	var files []*FileInfo

	err := c.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())

			// 只处理文件信息键
			if len(key) > len(fileInfoPrefix) && key[:len(fileInfoPrefix)] == fileInfoPrefix {
				fileKey := key[len(fileInfoPrefix):]
				err := item.Value(func(val []byte) error {
					fileInfo := &FileInfo{}
					if err := json.Unmarshal(val, fileInfo); err != nil {
						return err
					}
					fileInfo.Key = fileKey
					files = append(files, fileInfo)
					return nil
				})
				if err != nil {
					return err
				}
			}
		}
		return nil
	})

	return files, err
}

// GetInfo 获取文件信息
func (c *badgerCache) GetInfo(ctx context.Context, key string) (*FileInfo, error) {
	var fileInfo *FileInfo

	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(fileInfoPrefix + key))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			fileInfo = &FileInfo{}
			if err := json.Unmarshal(val, fileInfo); err != nil {
				return err
			}
			fileInfo.Key = key
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return fileInfo, nil
}

// Cleanup 清理过期文件
func (c *badgerCache) Cleanup(ctx context.Context) error {
	now := time.Now()
	var expiredFiles []string
	var totalSize int64

	// 找出过期文件
	err := c.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())

			if len(key) > len(fileInfoPrefix) && key[:len(fileInfoPrefix)] == fileInfoPrefix {
				fileKey := key[len(fileInfoPrefix):]
				err := item.Value(func(val []byte) error {
					fileInfo := &FileInfo{}
					if err := json.Unmarshal(val, fileInfo); err != nil {
						return err
					}
					if now.After(fileInfo.ExpiresAt) {
						expiredFiles = append(expiredFiles, fileKey)
						totalSize += fileInfo.Size
					}
					return nil
				})
				if err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// 删除过期文件
	for _, fileKey := range expiredFiles {
		if err := c.Delete(ctx, fileKey); err != nil {
			// 记录错误但继续清理其他文件
			continue
		}
	}

	// 更新统计信息
	c.mu.Lock()
	c.stats.ExpiredFiles = int64(len(expiredFiles))
	c.stats.LastCleanup = now
	c.mu.Unlock()

	// 保存统计信息
	c.saveStats()

	return nil
}

// Close 关闭缓存
func (c *badgerCache) Close() error {
	return c.db.Close()
}

// Stats 获取缓存统计信息
func (c *badgerCache) Stats() (*Stats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 创建统计信息副本
	stats := *c.stats
	return &stats, nil
}

// readCloser 实现io.ReadCloser接口
type readCloser struct {
	data []byte
	pos  int
}

func (r *readCloser) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *readCloser) Close() error {
	return nil
}
