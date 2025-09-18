package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/seraphico/EdgeOrigin/pkg/filecache"
)

func main() {
	// 创建配置
	config := &filecache.Config{
		DataDir:         "./test_cache",
		MaxCacheSize:    100 * 1024 * 1024, // 100MB
		DefaultTTL:      time.Hour,
		CleanupInterval: 10 * time.Minute,
		Compression:     true,
	}

	// 创建缓存
	cache, err := filecache.NewBadgerCache(config)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// 存储文件
	key := "test-file.txt"
	data := strings.NewReader("Hello, World! This is a test file.")
	err = cache.Set(ctx, key, data, "text/plain", time.Hour)
	if err != nil {
		log.Fatalf("Failed to set file: %v", err)
	}

	// 检查文件是否存在
	exists, err := cache.Exists(ctx, key)
	if err != nil {
		log.Fatalf("Failed to check existence: %v", err)
	}
	fmt.Printf("File exists: %v\n", exists)

	// 获取文件
	reader, fileInfo, err := cache.Get(ctx, key)
	if err != nil {
		log.Fatalf("Failed to get file: %v", err)
	}
	defer reader.Close()

	// 读取文件内容
	content, err := io.ReadAll(reader)
	if err != nil {
		log.Fatalf("Failed to read content: %v", err)
	}

	fmt.Printf("File content: %s\n", string(content))
	fmt.Printf("File info: %+v\n", fileInfo)

	// 获取文件信息
	info, err := cache.GetInfo(ctx, key)
	if err != nil {
		log.Fatalf("Failed to get file info: %v", err)
	}
	fmt.Printf("File info: %+v\n", info)

	// 列出所有文件
	files, err := cache.List(ctx)
	if err != nil {
		log.Fatalf("Failed to list files: %v", err)
	}
	fmt.Printf("Total files: %d\n", len(files))

	// 获取统计信息
	stats, err := cache.Stats()
	if err != nil {
		log.Fatalf("Failed to get stats: %v", err)
	}
	fmt.Printf("Cache stats: %+v\n", stats)

	// 删除文件
	err = cache.Delete(ctx, key)
	if err != nil {
		log.Fatalf("Failed to delete file: %v", err)
	}

	// 再次检查文件是否存在
	exists, err = cache.Exists(ctx, key)
	if err != nil {
		log.Fatalf("Failed to check existence: %v", err)
	}
	fmt.Printf("File exists after deletion: %v\n", exists)
}
