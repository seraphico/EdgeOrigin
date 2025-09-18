package filecache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// loadStats 加载统计信息
func (c *badgerCache) loadStats() error {
	return c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(statsKey))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				// 使用默认统计信息
				c.stats = &Stats{}
				return nil
			}
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, c.stats)
		})
	})
}

// saveStats 保存统计信息
func (c *badgerCache) saveStats() error {
	c.mu.RLock()
	stats := *c.stats
	c.mu.RUnlock()

	statsBytes, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	return c.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(statsKey), statsBytes)
	})
}

// updateStatsAfterSet 设置文件后更新统计
func (c *badgerCache) updateStatsAfterSet(size int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stats.TotalFiles++
	c.stats.TotalSize += size
}

// updateStatsAfterDelete 删除文件后更新统计
func (c *badgerCache) updateStatsAfterDelete(size int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stats.TotalFiles > 0 {
		c.stats.TotalFiles--
	}
	if c.stats.TotalSize >= size {
		c.stats.TotalSize -= size
	}
}

// updateStatsAfterHit 命中后更新统计
func (c *badgerCache) updateStatsAfterHit() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 简单的命中率计算
	total := c.stats.HitRate + c.stats.MissRate
	if total == 0 {
		c.stats.HitRate = 1.0
		c.stats.MissRate = 0.0
	} else {
		c.stats.HitRate = (c.stats.HitRate*total + 1) / (total + 1)
		c.stats.MissRate = 1 - c.stats.HitRate
	}
}

// updateStatsAfterMiss 未命中后更新统计
func (c *badgerCache) updateStatsAfterMiss() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 简单的命中率计算
	total := c.stats.HitRate + c.stats.MissRate
	if total == 0 {
		c.stats.HitRate = 0.0
		c.stats.MissRate = 1.0
	} else {
		c.stats.MissRate = (c.stats.MissRate*total + 1) / (total + 1)
		c.stats.HitRate = 1 - c.stats.MissRate
	}
}

// updateFileAccess 更新文件访问信息
func (c *badgerCache) updateFileAccess(key string, fileInfo *FileInfo) {
	// 更新访问次数和最后访问时间
	fileInfo.AccessCount++
	fileInfo.LastAccess = time.Now()

	// 保存更新后的文件信息
	infoBytes, err := json.Marshal(fileInfo)
	if err != nil {
		return
	}

	c.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(fileInfoPrefix+key), infoBytes)
	})
}

// startCleanupRoutine 启动清理协程
func (c *badgerCache) startCleanupRoutine() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if err := c.Cleanup(ctx); err != nil {
			// 记录错误但不中断清理协程
			continue
		}
		cancel()
	}
}
