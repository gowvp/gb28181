package recording

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/gowvp/owl/internal/notify"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/system"
	"github.com/ixugo/goddd/pkg/web"
	"gorm.io/gorm"
)

// StartCleanupWorker 启动定时清理协程
// 常规每 10 分钟检查一次，磁盘超标时缩短到 2 分钟加速清理
func (c Core) StartCleanupWorker() {
	if c.conf == nil || c.conf.Disabled {
		slog.Info("recording cleanup disabled")
		return
	}

	slog.Info("recording cleanup worker started",
		"retain_days", c.conf.RetainDays,
		"disk_threshold", c.conf.DiskUsageThreshold,
		"storage_dir", c.conf.StorageDir,
	)

	// 程序启动时先执行一次清理
	c.runCleanup()

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// 磁盘超标时连续重试，间隔 2 分钟，直到达标或无录像可删
		for {
			needRetry := c.runCleanup()
			if !needRetry {
				break
			}
			slog.Warn("磁盘仍超标，2 分钟后重试清理", "threshold", c.conf.DiskUsageThreshold)
			notify.Warn(fmt.Sprintf("磁盘使用率超标 (阈值: %.0f%%), 正在清理录像", c.conf.DiskUsageThreshold))
			time.Sleep(2 * time.Minute)
		}
	}
}

// runCleanup 执行清理流程：先预标记即将过期的录像，再清理过期录像，
// 然后清理文件系统上超龄的孤儿目录，最后处理磁盘空间
// 返回 true 表示磁盘仍超标，调用方应短间隔重试
func (c Core) runCleanup() bool {
	c.markExpiringRecordings()
	c.cleanupExpiredRecordings()
	c.cleanupOrphanDirs()
	return c.cleanupByDiskUsage()
}

// markExpiringRecordings 预标记 1 小时内即将过期的录像
func (c Core) markExpiringRecordings() {
	if c.conf.RetainDays <= 0 {
		return
	}

	ctx := context.Background()
	// 计算 1 小时后的过期时间阈值
	// 如果录像的 started_at < (now + 1h - retain_days)，则该录像将在 1 小时内过期
	expiryCutoff := time.Now().Add(time.Hour).AddDate(0, 0, -c.conf.RetainDays)

	// 批量更新 delete_flag
	err := c.store.Recording().Session(ctx, func(tx *gorm.DB) error {
		return tx.Model(&Recording{}).
			Where("delete_flag = ?", false).
			Where("started_at < ?", orm.Time{Time: expiryCutoff}).
			Update("delete_flag", true).Error
	})
	if err != nil {
		slog.Warn("failed to mark expiring recordings", "err", err)
	}
}

// cleanupExpiredRecordings 清理超过保留天数的录像
func (c Core) cleanupExpiredRecordings() {
	if c.conf.RetainDays <= 0 {
		return
	}

	ctx := context.Background()
	cutoffTime := time.Now().AddDate(0, 0, -c.conf.RetainDays)

	totalDeleted, filesDeleted, freedBytes, failedFiles := c.batchDeleteRecordings(ctx,
		"expired",
		orm.Where("started_at < ?", orm.Time{Time: cutoffTime}),
	)

	if totalDeleted > 0 || failedFiles > 0 {
		slog.Info("expired recording cleanup completed",
			"reason", "retention_policy",
			"retain_days", c.conf.RetainDays,
			"cutoff_time", cutoffTime.Format(time.DateTime),
			"recordings_deleted", totalDeleted,
			"files_deleted", filesDeleted,
			"failed_files", failedFiles,
			"freed_bytes", freedBytes,
		)
	}
}

// cleanupByDiskUsage 基于磁盘使用率清理录像
// 循环删除最旧的录像，每删一批重新检查磁盘使用率，直到降到阈值以下
// 返回 true 表示磁盘仍超标，调用方应短间隔重试
func (c Core) cleanupByDiskUsage() bool {
	if c.conf.DiskUsageThreshold <= 0 || c.conf.DiskUsageThreshold >= 100 {
		return false
	}

	storageDir := c.conf.StorageDir
	if storageDir == "" {
		storageDir = "./recordings"
	}

	absStorageDir := filepath.Join(system.Getwd(), storageDir)
	if _, err := os.Stat(absStorageDir); os.IsNotExist(err) {
		return false
	}

	usage, err := getDiskUsage(absStorageDir)
	if err != nil {
		slog.Warn("获取磁盘使用率失败", "err", err)
		return false
	}

	if usage < c.conf.DiskUsageThreshold {
		return false
	}

	slog.Info("磁盘使用率超标，开始清理录像", "current_usage", usage, "threshold", c.conf.DiskUsageThreshold)

	ctx := context.Background()
	var totalFreed int64
	var deletedCount, failedCount int
	batchSize := 50
	maxBatches := 20
	reachedMax := false

	// 持续删除最旧录像，每批删完重新检查磁盘，直到达标或无录像可删
	for batch := 0; batch < maxBatches; batch++ {
		usage, err = getDiskUsage(absStorageDir)
		if err != nil || usage < c.conf.DiskUsageThreshold {
			break
		}

		var oldestRecordings []*Recording
		pager := web.PagerFilter{Page: 1, Size: batchSize}
		_, err := c.store.Recording().List(ctx, &oldestRecordings, &pager,
			orm.OrderBy("started_at ASC"),
		)
		if err != nil || len(oldestRecordings) == 0 {
			break
		}

		// 只有文件成功删除（或已不存在）才加入 deleteIDs，避免孤儿文件
		var deleteIDs []int64
		var batchFreed int64
		var batchFailed int
		var fileNames []string

		// 收集本批待删除的录像明细
		type diskDeleteDetail struct {
			ID        int64  `json:"id"`
			CID       string `json:"cid"`
			Path      string `json:"path"`
			StartedAt string `json:"started_at"`
			Size      int64  `json:"size"`
			Status    string `json:"status"`
		}
		var details []diskDeleteDetail

		for _, rec := range oldestRecordings {
			filePath := rec.Path
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(system.Getwd(), filePath)
			}
			if err := os.Remove(filePath); err != nil {
				if os.IsNotExist(err) {
					deleteIDs = append(deleteIDs, rec.ID)
					fileNames = append(fileNames, filepath.Base(filePath))
					details = append(details, diskDeleteDetail{rec.ID, rec.CID, filePath, rec.StartedAt.Format(time.DateTime), rec.Size, "not_exist"})
				} else {
					batchFailed++
					details = append(details, diskDeleteDetail{rec.ID, rec.CID, filePath, rec.StartedAt.Format(time.DateTime), rec.Size, "failed"})
					slog.Warn("文件删除失败，跳过", "path", filePath, "err", err)
				}
			} else {
				batchFreed += rec.Size
				deleteIDs = append(deleteIDs, rec.ID)
				fileNames = append(fileNames, filepath.Base(filePath))
				details = append(details, diskDeleteDetail{rec.ID, rec.CID, filePath, rec.StartedAt.Format(time.DateTime), rec.Size, "deleted"})
			}
		}

		// 一条日志汇总本批所有删除明细
		slog.Info("磁盘清理批次",
			"batch", batch+1,
			"total", len(oldestRecordings),
			"to_delete", len(deleteIDs),
			"failed", batchFailed,
			"freed_bytes", batchFreed,
			"details", details,
		)

		// 批量删除数据库记录
		if len(deleteIDs) > 0 {
			if err := c.store.Recording().Session(ctx, func(tx *gorm.DB) error {
				return tx.Where("id IN ?", deleteIDs).Delete(&Recording{}).Error
			}); err != nil {
				slog.Warn("数据库记录删除失败，文件已删除但记录残留", "count", len(deleteIDs), "err", err)
				// DB 删失败，这些记录下次会再查到，走 IsNotExist 分支清理，不影响正确性
			} else {
				deletedCount += len(deleteIDs)
			}
		}

		totalFreed += batchFreed
		failedCount += batchFailed

		// 达到批次上限，标记后退出，由外层定时重试
		if batch == maxBatches-1 {
			reachedMax = true
		}
	}

	if reachedMax {
		slog.Warn("磁盘清理达到批次上限，2 分钟后重试", "max_batches", maxBatches, "deleted", deletedCount, "failed", failedCount)
	}

	// DB 已无可删录像但磁盘仍超标时，按文件系统最旧优先删除孤儿目录
	if !reachedMax {
		if u, e := getDiskUsage(absStorageDir); e == nil && u >= c.conf.DiskUsageThreshold {
			c.cleanupDiskByFilesystem(absStorageDir)
		}
	}

	// 清理空目录
	cleanupEmptyDirs(absStorageDir)

	// 预标记已释放量的 2 倍，为下次清理预留缓冲，避免下次刚触发就立刻要删大量文件
	c.markNextDeletionCandidates(ctx, totalFreed*2)

	if deletedCount > 0 || failedCount > 0 {
		slog.Info("磁盘清理完成",
			"reason", "disk_threshold_exceeded",
			"current_usage", usage,
			"threshold", c.conf.DiskUsageThreshold,
			"recordings_deleted", deletedCount,
			"failed_files", failedCount,
			"freed_bytes", totalFreed,
		)
	}

	// 返回磁盘是否仍超标，供调用方决定是否加速重试
	finalUsage, err := getDiskUsage(absStorageDir)
	return err == nil && finalUsage >= c.conf.DiskUsageThreshold
}

// markNextDeletionCandidates 预标记即将被删除的录像
// 标记最旧的、总大小约等于 targetSize 的录像为待删除状态
func (c Core) markNextDeletionCandidates(ctx context.Context, targetSize int64) {
	if targetSize <= 0 {
		return
	}

	// 查询未被标记的最旧录像
	var candidates []*Recording
	pager := web.PagerFilter{Page: 1, Size: 200}
	_, err := c.store.Recording().List(ctx, &candidates, &pager,
		orm.Where("delete_flag = ?", false),
		orm.OrderBy("started_at ASC"),
	)
	if err != nil || len(candidates) == 0 {
		return
	}

	// 计算需要标记的录像
	var markedSize int64
	var markIDs []int64
	for _, rec := range candidates {
		if markedSize >= targetSize {
			break
		}
		markIDs = append(markIDs, rec.ID)
		markedSize += rec.Size
	}

	// 批量更新标记
	if len(markIDs) > 0 {
		_ = c.store.Recording().Session(ctx, func(tx *gorm.DB) error {
			return tx.Model(&Recording{}).Where("id IN ?", markIDs).Update("delete_flag", true).Error
		})
	}
}

// batchDeleteRecordings 批量删除录像（文件+数据库记录）
// 只有文件成功删除（或已不存在）才删数据库记录，避免孤儿文件
func (c Core) batchDeleteRecordings(ctx context.Context, reason string, conditions ...orm.QueryOption) (totalDeleted, filesDeleted, failedFiles int, freedBytes int64) {
	batchSize := 100

	for {
		var recordings []*Recording
		pager := web.PagerFilter{Page: 1, Size: batchSize}
		_, err := c.store.Recording().List(ctx, &recordings, &pager, conditions...)
		if err != nil || len(recordings) == 0 {
			break
		}

		var deleteIDs []int64
		var batchFreed int64
		var batchFilesDeleted, batchFailed int
		var fileNames []string

		// 收集本批待删除的录像明细，用于一条日志汇总
		type deleteDetail struct {
			ID        int64  `json:"id"`
			CID       string `json:"cid"`
			Path      string `json:"path"`
			StartedAt string `json:"started_at"`
			Size      int64  `json:"size"`
			Status    string `json:"status"` // deleted / not_exist / failed
		}
		var details []deleteDetail

		for _, rec := range recordings {
			filePath := rec.Path
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(system.Getwd(), filePath)
			}
			if err := os.Remove(filePath); err != nil {
				if os.IsNotExist(err) {
					deleteIDs = append(deleteIDs, rec.ID)
					fileNames = append(fileNames, filepath.Base(filePath))
					details = append(details, deleteDetail{rec.ID, rec.CID, filePath, rec.StartedAt.Format(time.DateTime), rec.Size, "not_exist"})
				} else {
					batchFailed++
					details = append(details, deleteDetail{rec.ID, rec.CID, filePath, rec.StartedAt.Format(time.DateTime), rec.Size, "failed"})
					slog.Warn("文件删除失败，跳过", "path", filePath, "err", err)
				}
			} else {
				batchFilesDeleted++
				batchFreed += rec.Size
				deleteIDs = append(deleteIDs, rec.ID)
				fileNames = append(fileNames, filepath.Base(filePath))
				details = append(details, deleteDetail{rec.ID, rec.CID, filePath, rec.StartedAt.Format(time.DateTime), rec.Size, "deleted"})
			}
		}

		// 一条日志汇总本批所有删除明细
		slog.Info("录像清理批次",
			"reason", reason,
			"total", len(recordings),
			"to_delete", len(deleteIDs),
			"failed", batchFailed,
			"freed_bytes", batchFreed,
			"details", details,
		)

		// 批量删除数据库记录
		if len(deleteIDs) > 0 {
			if err := c.store.Recording().Session(ctx, func(tx *gorm.DB) error {
				return tx.Where("id IN ?", deleteIDs).Delete(&Recording{}).Error
			}); err != nil {
				slog.Warn("数据库记录删除失败", "count", len(deleteIDs), "err", err)
			} else {
				totalDeleted += len(deleteIDs)
			}
		}

		filesDeleted += batchFilesDeleted
		failedFiles += batchFailed
		freedBytes += batchFreed
	}

	// 清理空目录
	if c.conf != nil && c.conf.StorageDir != "" {
		absStorageDir := filepath.Join(system.Getwd(), c.conf.StorageDir)
		cleanupEmptyDirs(absStorageDir)
	}

	return
}

// getDiskUsage 获取指定路径所在磁盘的使用率（百分比）
func getDiskUsage(path string) (float64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	if total == 0 {
		return 0, nil
	}

	usage := float64(used) / float64(total) * 100
	return usage, nil
}

// getAbsStorageDir 返回录像存储目录的绝对路径
func (c Core) getAbsStorageDir() string {
	storageDir := c.conf.StorageDir
	if storageDir == "" {
		storageDir = "./recordings"
	}
	return filepath.Join(system.Getwd(), storageDir)
}

// dateDirEntry 文件系统上发现的日期目录
type dateDirEntry struct {
	path string
	date time.Time
}

// scanDateDirs 递归扫描录像目录树，收集所有 YYYY-MM-DD 格式的日期目录
func scanDateDirs(root string) []dateDirEntry {
	var result []dateDirEntry
	var scan func(dir string)
	scan = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			t, err := time.Parse("2006-01-02", e.Name())
			if err != nil {
				scan(filepath.Join(dir, e.Name()))
				continue
			}
			result = append(result, dateDirEntry{
				path: filepath.Join(dir, e.Name()),
				date: t,
			})
		}
	}
	scan(root)
	return result
}

// cleanupOrphanDirs 按保留天数扫描文件系统，删除超龄的日期目录
// 作为数据库清理的兜底：即使录像记录不在数据库中，
// 只要日期目录超龄就会被清理，避免孤儿文件无限累积
func (c Core) cleanupOrphanDirs() {
	if c.conf.RetainDays <= 0 {
		return
	}

	absStorageDir := c.getAbsStorageDir()
	if _, err := os.Stat(absStorageDir); os.IsNotExist(err) {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -c.conf.RetainDays)
	dirs := scanDateDirs(absStorageDir)

	var removedCount int
	for _, d := range dirs {
		if !d.date.Before(cutoff) {
			continue
		}
		if err := os.RemoveAll(d.path); err != nil {
			slog.Warn("删除过期日期目录失败", "path", d.path, "err", err)
			continue
		}
		removedCount++
		slog.Info("删除过期日期目录", "path", d.path, "date", d.date.Format("2006-01-02"))
	}
	if removedCount > 0 {
		slog.Info("过期目录清理完成", "removed_dirs", removedCount)
		cleanupEmptyDirs(absStorageDir)
	}
}

// cleanupDiskByFilesystem 文件系统级磁盘清理兜底
// 当数据库中已无可删录像但磁盘仍超标时，直接按日期目录从旧到新逐个删除
func (c Core) cleanupDiskByFilesystem(absStorageDir string) {
	dirs := scanDateDirs(absStorageDir)
	if len(dirs) == 0 {
		return
	}

	slices.SortFunc(dirs, func(a, b dateDirEntry) int {
		return a.date.Compare(b.date)
	})

	slog.Info("数据库无可删录像，启动文件系统级磁盘清理", "date_dirs", len(dirs))
	for _, d := range dirs {
		usage, err := getDiskUsage(absStorageDir)
		if err != nil || usage < c.conf.DiskUsageThreshold {
			break
		}
		if err := os.RemoveAll(d.path); err != nil {
			slog.Warn("文件系统磁盘清理: 删除目录失败", "path", d.path, "err", err)
			continue
		}
		slog.Info("文件系统磁盘清理: 删除目录", "path", d.path, "date", d.date.Format("2006-01-02"))
	}
	cleanupEmptyDirs(absStorageDir)
}

// cleanupEmptyDirs 递归删除空目录
func cleanupEmptyDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(dir, entry.Name())
			cleanupEmptyDirs(subDir)

			// 检查子目录是否为空
			subEntries, err := os.ReadDir(subDir)
			if err == nil && len(subEntries) == 0 {
				_ = os.Remove(subDir)
			}
		}
	}
}
