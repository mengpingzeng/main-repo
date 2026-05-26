package c2_dashboard

// DashboardQueryRequest 看板查询入参（所有字段可选）。
type DashboardQueryRequest struct {
	UID        string   `json:"uid,omitempty"`
	TaskID     string   `json:"taskId,omitempty"`
	AccountIDs []string `json:"accountIds,omitempty"`
	Platforms  []string `json:"platforms,omitempty"`
	SkillIDs   []string `json:"skillIds,omitempty"`
	SessionIDs []string `json:"sessionIds,omitempty"`
	StartTime  string   `json:"startTime,omitempty"` // RFC3339
	EndTime    string   `json:"endTime,omitempty"`   // RFC3339
	Page       int      `json:"page,omitempty"`
	Size       int      `json:"size,omitempty"`
}

// DashboardQueryResponse 看板查询出参。
type DashboardQueryResponse struct {
	Items   []DashboardItem  `json:"items"`
	Summary DashboardSummary `json:"summary"`
	Total   int              `json:"total"`
}

// DashboardItem 单条看板条目。
type DashboardItem struct {
	PostID      string `json:"postId"`
	AccountID   string `json:"accountId"`
	Platform    string `json:"platform"`
	SkillID     string `json:"skillId,omitempty"`
	SessionID   string `json:"sessionId,omitempty"`
	NovelName   string `json:"novelName,omitempty"`
	LoginName   string `json:"loginName,omitempty"`
	Views       int    `json:"views"`
	Likes       int    `json:"likes"`
	Comments    int    `json:"comments"`
	Shares      int    `json:"shares"`
	PublishedAt string `json:"publishedAt"`
}

// DashboardSummary 聚合汇总（本次查询结果集的纯总量，不返回均值）。
type DashboardSummary struct {
	TotalPosts    int `json:"totalPosts"`
	TotalViews    int `json:"totalViews"`
	TotalLikes    int `json:"totalLikes"`
	TotalComments int `json:"totalComments"`
	TotalShares   int `json:"totalShares"`
}
