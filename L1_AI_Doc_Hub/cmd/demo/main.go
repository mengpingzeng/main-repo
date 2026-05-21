package main

import (
	"context"
	"fmt"

	"a4md"
)

func main() {
	storage, err := a4md.NewLocalFSStorage("/tmp/a4md_data")
	if err != nil {
		panic(err)
	}

	engine, err := a4md.NewTemplateEngine("v1")
	if err != nil {
		panic(err)
	}

	svc := a4md.NewService(storage, nil, engine, nil)

	input := a4md.WriteMDInput{
		TaskID:       "task_demo_001",
		UID:          "user_demo_001",
		Topic:        "小龙虾测评",
		SkillID:      "xhs_grass_v1",
		SkillName:    "小红书种草",
		SkillVersion: "2.1.0",
		Model:        "deepseek-chat",
		DraftVersion: 3,
		Products: a4md.Products{
			XhsText:    "小龙虾太香啦！",
			WechatHTML: "<p>深度测评</p>",
		},
		Sessions: []a4md.SessionInfo{
			{
				SessionID:    "sess_aaa",
				MessageCount: 15,
				DraftVersion: 3,
			},
		},
		PublishResults: []a4md.PublishResult{
			{AccountID: "acc_001", Platform: "xhs", Status: "ok", PostID: "p123"},
		},
	}

	result, err := svc.WriteMD(context.Background(), input)
	if err != nil {
		panic(err)
	}

	fmt.Println("档案路径:", result.MDPath)

	err = svc.AppendStats(context.Background(), a4md.AppendStatsInput{
		TaskID:       "task_demo_001",
		StatsPeriod:  "24h",
		DraftVersion: 3,
		Stats: []a4md.StatItem{
			{AccountID: "acc_001", Platform: "xhs", Views: 1523, Likes: 89},
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("追加完成，档案路径:", result.MDPath)
}
