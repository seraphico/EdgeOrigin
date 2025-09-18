# 文件缓存系统

基于 BadgerDB 实现的高性能文件缓存系统，支持文件存储、检索、过期管理和统计信息。

## 特性

- 🚀 **高性能**: 基于 BadgerDB 的 LSM-Tree 存储引擎
- 📁 **文件管理**: 支持任意类型文件的存储和检索
- ⏰ **自动过期**: 支持 TTL 和自动清理机制
- 📊 **统计信息**: 提供详细的缓存统计和监控
- 🔧 **可配置**: 灵活的配置选项
- 🛡️ **类型安全**: 完整的 Go 类型系统支持

## 快速开始

### 基本使用

```go
package main

import (
    "context"
    "fmt"
    "strings"
    "time"
    
    "github.com/seraphico/EdgeOrigin/pkg/filecache"
)

func main() {
    // 创建配置
    config := &filecache.Config{
        DataDir:         "./cache",
        MaxCacheSize:    1024 * 1024 * 1024, // 1GB
        DefaultTTL:      24 * time.Hour,
        CleanupInterval: time.Hour,
        Compression:     true,
    }

    // 创建缓存实例
    cache, err := filecache.NewBadgerCache(config)
    if err != nil {
        panic(err)
    }
    defer cache.Close()

    ctx := context.Background()

    // 存储文件
    key := "my-file.txt"
    data := strings.NewReader("Hello, World!")
    err = cache.Set(ctx, key, data, "text/plain", time.Hour)
    if err != nil {
        panic(err)
    }

    // 获取文件
    reader, fileInfo, err := cache.Get(ctx, key)
    if err != nil {
        panic(err)
    }
    defer reader.Close()

    // 读取内容
    content, _ := io.ReadAll(reader)
    fmt.Printf("Content: %s\n", string(content))
    fmt.Printf("File info: %+v\n", fileInfo)
}
```

### 使用默认配置

```go
// 使用默认配置
cache, err := filecache.NewBadgerCache(nil)
if err != nil {
    panic(err)
}
defer cache.Close()
```

### 从配置文件加载

```go
// 从文件加载配置
config, err := filecache.LoadConfigFromFile("cache-config.json")
if err != nil {
    panic(err)
}

cache, err := filecache.NewCacheWithConfig(config)
if err != nil {
    panic(err)
}
defer cache.Close()
```

## API 参考

### Cache 接口

```go
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
```

### 配置选项

```go
type Config struct {
    DataDir         string        `json:"data_dir"`          // 数据目录
    MaxCacheSize    int64         `json:"max_cache_size"`    // 最大缓存大小（字节）
    DefaultTTL      time.Duration `json:"default_ttl"`       // 默认TTL
    CleanupInterval time.Duration `json:"cleanup_interval"`  // 清理间隔
    Compression     bool          `json:"compression"`       // 是否压缩
}
```

### 文件信息

```go
type FileInfo struct {
    Key         string    `json:"key"`          // 缓存键
    Size        int64     `json:"size"`         // 文件大小
    MimeType    string    `json:"mime_type"`    // MIME类型
    CreatedAt   time.Time `json:"created_at"`   // 创建时间
    ExpiresAt   time.Time `json:"expires_at"`   // 过期时间
    AccessCount int64     `json:"access_count"` // 访问次数
    LastAccess  time.Time `json:"last_access"`  // 最后访问时间
}
```

## 高级用法

### 批量操作

```go
// 批量存储文件
files := map[string]string{
    "file1.txt": "Content 1",
    "file2.txt": "Content 2",
    "file3.txt": "Content 3",
}

for key, content := range files {
    err := cache.Set(ctx, key, strings.NewReader(content), "text/plain", time.Hour)
    if err != nil {
        log.Printf("Failed to store %s: %v", key, err)
    }
}

// 列出所有文件
fileList, err := cache.List(ctx)
if err != nil {
    panic(err)
}

for _, file := range fileList {
    fmt.Printf("File: %s, Size: %d, Type: %s\n", 
        file.Key, file.Size, file.MimeType)
}
```

### 监控和统计

```go
// 获取统计信息
stats, err := cache.Stats()
if err != nil {
    panic(err)
}

fmt.Printf("Total files: %d\n", stats.TotalFiles)
fmt.Printf("Total size: %d bytes\n", stats.TotalSize)
fmt.Printf("Hit rate: %.2f%%\n", stats.HitRate*100)
fmt.Printf("Miss rate: %.2f%%\n", stats.MissRate*100)
fmt.Printf("Expired files: %d\n", stats.ExpiredFiles)
```

### 自定义过期策略

```go
// 设置不同的TTL
cache.Set(ctx, "short-lived", data1, "text/plain", time.Minute)
cache.Set(ctx, "long-lived", data2, "text/plain", time.Hour*24)
cache.Set(ctx, "permanent", data3, "text/plain", time.Hour*24*365)

// 手动清理过期文件
err := cache.Cleanup(ctx)
if err != nil {
    log.Printf("Cleanup failed: %v", err)
}
```

## 性能优化

1. **启用压缩**: 设置 `Compression: true` 可以减少存储空间
2. **合理设置缓存大小**: 根据可用内存设置 `MaxCacheSize`
3. **定期清理**: 设置合适的 `CleanupInterval` 避免过期文件积累
4. **批量操作**: 尽量批量处理文件以提高效率

## 注意事项

1. **资源管理**: 使用完毕后务必调用 `Close()` 方法
2. **并发安全**: 缓存实例是并发安全的，可以在多个 goroutine 中使用
3. **错误处理**: 所有操作都可能返回错误，请妥善处理
4. **内存使用**: 大文件会占用较多内存，请根据实际情况调整配置

## 测试

运行测试：

```bash
go test ./pkg/filecache/...
```

运行示例：

```bash
go run ./pkg/filecache/example_test.go
```
