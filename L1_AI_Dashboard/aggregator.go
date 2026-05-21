package c2_dashboard

// computeSummary 基于 items 生成本次查询的汇总（纯总量，不返回均值）。
func computeSummary(items []DashboardItem) DashboardSummary {
	s := DashboardSummary{TotalPosts: len(items)}
	for _, item := range items {
		s.TotalViews += item.Views
		s.TotalLikes += item.Likes
		s.TotalComments += item.Comments
		s.TotalShares += item.Shares
	}
	return s
}
