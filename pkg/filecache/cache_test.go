package filecache

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestBadgerCache(t *testing.T) {
	// 创建测试配置
	config := &Config{
		DataDir:         "./test_cache",
		MaxCacheSize:    1024 * 1024, // 1MB
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
		Compression:     true,
	}

	// 创建缓存
	cache, err := NewBadgerCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	t.Run("Set and Get", func(t *testing.T) {
		key := "test-file.txt"
		data := strings.NewReader("Hello, World!")

		// 存储文件
		err := cache.Set(ctx, key, data, "text/plain", time.Hour)
		if err != nil {
			t.Fatalf("Failed to set file: %v", err)
		}

		// 获取文件
		reader, fileInfo, err := cache.Get(ctx, key)
		if err != nil {
			t.Fatalf("Failed to get file: %v", err)
		}
		defer reader.Close()

		// 验证文件内容
		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("Failed to read content: %v", err)
		}

		if string(content) != "Hello, World!" {
			t.Errorf("Expected 'Hello, World!', got '%s'", string(content))
		}

		// 验证文件信息
		if fileInfo.Key != key {
			t.Errorf("Expected key %s, got %s", key, fileInfo.Key)
		}
		if fileInfo.MimeType != "text/plain" {
			t.Errorf("Expected MIME type 'text/plain', got '%s'", fileInfo.MimeType)
		}
		if fileInfo.Size != 13 {
			t.Errorf("Expected size 13, got %d", fileInfo.Size)
		}
	})

	t.Run("List", func(t *testing.T) {
		// 添加一些测试文件
		files := []struct {
			key      string
			content  string
			mimeType string
		}{
			{"file1.txt", "Content 1", "text/plain"},
			{"file2.txt", "Content 2", "text/plain"},
			{"file3.json", `{"key": "value"}`, "application/json"},
		}

		for _, file := range files {
			err := cache.Set(ctx, file.key, strings.NewReader(file.content), file.mimeType, time.Hour)
			if err != nil {
				t.Fatalf("Failed to set file %s: %v", file.key, err)
			}
		}

		// 列出所有文件
		fileList, err := cache.List(ctx)

		for idex, file := range fileList {
			t.Logf("fileList[%d]: %+v", idex, file.Key)
		}

		if err != nil {
			t.Fatalf("Failed to list files: %v", err)
		}

		if len(fileList) != len(files) {
			t.Errorf("Expected %d files, got %d", len(files), len(fileList))
		}

		// 验证文件信息
		fileMap := make(map[string]*FileInfo)
		for _, file := range fileList {
			fileMap[file.Key] = file
		}

		for _, expectedFile := range files {
			actualFile, exists := fileMap[expectedFile.key]
			if !exists {
				t.Errorf("File %s not found in list", expectedFile.key)
				continue
			}
			if actualFile.MimeType != expectedFile.mimeType {
				t.Errorf("Expected MIME type %s for %s, got %s", expectedFile.mimeType, expectedFile.key, actualFile.MimeType)
			}
		}
	})

	t.Run("Exists", func(t *testing.T) {
		key := "test-file.txt"

		// 检查存在的文件
		exists, err := cache.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Failed to check existence: %v", err)
		}
		if !exists {
			t.Error("Expected file to exist")
		}

		// 检查不存在的文件
		exists, err = cache.Exists(ctx, "non-existent.txt")
		if err != nil {
			t.Fatalf("Failed to check existence: %v", err)
		}
		if exists {
			t.Error("Expected file to not exist")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		key := "test-file.txt"

		// 删除文件
		err := cache.Delete(ctx, key)
		if err != nil {
			t.Fatalf("Failed to delete file: %v", err)
		}

		// 验证文件已被删除
		exists, err := cache.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Failed to check existence: %v", err)
		}
		if exists {
			t.Error("Expected file to be deleted")
		}
	})

	t.Run("Stats", func(t *testing.T) {
		stats, err := cache.Stats()
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if stats.TotalFiles < 0 {
			t.Error("Total files should not be negative")
		}
		if stats.TotalSize < 0 {
			t.Error("Total size should not be negative")
		}
	})

	t.Run("Expiration", func(t *testing.T) {
		key := "expire-test.txt"
		data := strings.NewReader("This will expire")

		// 设置短TTL
		err := cache.Set(ctx, key, data, "text/plain", time.Millisecond*100)
		if err != nil {
			t.Fatalf("Failed to set file: %v", err)
		}

		// 立即获取应该成功
		_, _, err = cache.Get(ctx, key)
		if err != nil {
			t.Fatalf("Failed to get file immediately: %v", err)
		}

		// 等待过期
		time.Sleep(time.Millisecond * 200)

		// 获取应该失败
		_, _, err = cache.Get(ctx, key)
		if err == nil {
			t.Error("Expected file to be expired")
		}

		// 手动清理
		err = cache.Cleanup(ctx)
		if err != nil {
			t.Fatalf("Failed to cleanup: %v", err)
		}
	})
}

func TestConfig(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultConfig()
		if config == nil {
			t.Fatal("Default config should not be nil")
		}
		if config.DataDir == "" {
			t.Error("Data directory should not be empty")
		}
		if config.MaxCacheSize <= 0 {
			t.Error("Max cache size should be positive")
		}
		if config.DefaultTTL <= 0 {
			t.Error("Default TTL should be positive")
		}
	})

	t.Run("ValidateConfig", func(t *testing.T) {
		// 测试有效配置
		validConfig := &Config{
			DataDir:         "./test",
			MaxCacheSize:    1024,
			DefaultTTL:      time.Hour,
			CleanupInterval: time.Minute,
		}
		if err := ValidateConfig(validConfig); err != nil {
			t.Errorf("Valid config should not error: %v", err)
		}

		// 测试无效配置
		invalidConfigs := []*Config{
			nil,
			{DataDir: ""},
			{DataDir: "./test", MaxCacheSize: 0},
			{DataDir: "./test", MaxCacheSize: 1024, DefaultTTL: 0},
			{DataDir: "./test", MaxCacheSize: 1024, DefaultTTL: time.Hour, CleanupInterval: 0},
		}

		for i, config := range invalidConfigs {
			if err := ValidateConfig(config); err == nil {
				t.Errorf("Invalid config %d should error", i)
			}
		}
	})
}
