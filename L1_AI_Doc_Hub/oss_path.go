package a4md

import (
	"fmt"
)

const (
	maxFileSize = 5 * 1024 * 1024 // 5MB
)

// buildOSSPath 构造规范的 OSS 路径
// 格式：tasks/{task_id}/archive/draft_v{draft_version}.md
// 分片格式：tasks/{task_id}/archive/draft_v{draft_version}_part{N}.md
func buildOSSPath(taskID string, draftVersion int, partNo int) string {
	suffix := ""
	if partNo > 0 {
		suffix = fmt.Sprintf("_part%d", partNo)
	}

	return fmt.Sprintf("tasks/%s/archive/draft_v%d%s.md", taskID, draftVersion, suffix)
}
