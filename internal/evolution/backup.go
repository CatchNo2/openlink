package evolution

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// backupManager 在自主变更前为文件创建备份，支持后续回滚。
type backupManager struct {
	rootDir  string
	backDir  string
	mu       sync.Mutex
}

var backups *backupManager

func initBackup(rootDir string) {
	backups = &backupManager{
		rootDir: rootDir,
		backDir: filepath.Join(rootDir, ".openlink", "backups"),
	}
	_ = os.MkdirAll(backups.backDir, 0755)
}

// Backup 在修改 path 之前调用：若文件存在则复制一份到备份目录，返回备份路径（可能为空）。
func (b *backupManager) Backup(path string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil // 文件尚不存在，无需备份
	}
	if err := os.MkdirAll(b.backDir, 0755); err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102-150405")
	rel, _ := filepath.Rel(b.rootDir, path)
	safe := filepath.Join(b.backDir, fmt.Sprintf("%s__%s", stamp, filepath.ToSlash(rel)))
	if err := os.MkdirAll(filepath.Dir(safe), 0755); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(safe, data, 0644); err != nil {
		return "", err
	}
	return safe, nil
}

// Restore 将备份文件还原到原路径。
func (b *backupManager) Restore(backupPath string) error {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}
	// 从文件名还原原始相对路径：去掉 "stamp__" 前缀
	base := filepath.Base(backupPath)
	idx := indexOf(base, "__")
	if idx < 0 {
		return fmt.Errorf("无法解析备份文件名: %s", base)
	}
	rel := base[idx+2:]
	orig := filepath.Join(b.rootDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(orig), 0755); err != nil {
		return err
	}
	return os.WriteFile(orig, data, 0644)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// backupFile 便捷函数：对文件（相对于 rootDir 的路径）做备份。
func backupFile(rootDir, relPath string) string {
	if backups == nil {
		initBackup(rootDir)
	}
	back, err := backups.Backup(filepath.Join(rootDir, relPath))
	if err != nil {
		return ""
	}
	return back
}
