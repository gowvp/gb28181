package recording

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gowvp/owl/internal/conf"
)

// 验证保留期边界日（恰好 RetainDays 天前）的目录不被误删，再超龄一天即删
func TestCleanupOrphanDirsBoundary(t *testing.T) {
	t.Chdir(t.TempDir())

	boundary := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	expired := time.Now().AddDate(0, 0, -8).Format("2006-01-02")
	for _, d := range []string{boundary, expired} {
		dir := filepath.Join("recordings", d)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "a.mp4"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	c := Core{conf: &conf.ServerRecording{RetainDays: 7}}
	c.cleanupOrphanDirs()

	if _, err := os.Stat(filepath.Join("recordings", boundary)); err != nil {
		t.Fatalf("边界日目录应保留: %v", err)
	}
	if _, err := os.Stat(filepath.Join("recordings", expired)); !os.IsNotExist(err) {
		t.Fatalf("超龄日目录应删除, err=%v", err)
	}
}
