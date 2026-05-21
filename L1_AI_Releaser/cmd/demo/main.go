// cmd/demo 演示 C1 发布模块的完整使用流程。
//
// 使用 Mock 模式（不依赖 A1、平台 API、MySQL）验证全流程。
// 运行方式：go run ./cmd/demo/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"clawstudios/l1_ai_releaser/services/a1_account"
	"clawstudios/l1_ai_releaser/services/c1_publisher"
)

func main() {
	ctx := context.Background()

	vault := a1_account.NewMockSecretVault()

	acc3, _ := vault.Bind(ctx, a1_account.BindRequest{
		UID: "user_2", Platform: "fanqie",
		CredentialsPlaintext: `{"access_token":"fj_tok","author_id":"au_456"}`,
		Caller: "bff",
	})

	fmt.Printf("绑定账号: %s\n", acc3.AccountID)

	a1Server := newMockA1Server(vault)
	defer a1Server.Close()

	fanqieAdapter := c1_publisher.NewMockPublishAdapter("fanqie")

	publisher := c1_publisher.NewRealPublisher(c1_publisher.Config{
		A1BaseURL: a1Server.URL,
		Adapters: []c1_publisher.PublishAdapter{
			fanqieAdapter,
		},
	})

	// 4. 模拟上游产出的文案内容
	chapterContent := "　　天快黑了。\n\n　　周明远蹲在工地门口的水泥地上，手里捏着一根已经抽到过滤嘴的烟，看着远处高楼上最后一抹夕阳被灰蒙蒙的雾气吞没。身后的搅拌机还在轰鸣，水泥灰被晚风卷起来，迷得人睁不开眼。他今年三十二岁，在工地上干了整整十二年，从砌砖的小工干到了施工员，一个月能拿七千块钱。但就在今天下午，工地老板跑了。\n\n　　「跑了就跑了吧。」旁边的老黄把安全帽摘下来，往地上一摔，溅起一蓬灰，「三个月没发工资了，老子早就该走了。」老黄今年五十八，头发白了大半，背也驼了。他在这个工地上干了快二十年，从九几年就开始跟着老板干活，说老板是好人，从来不拖欠工资。可这一次，老板连人带钱一起消失了。\n\n　　周明远没有搭话。他把烟头摁灭，抬头看了看天。城市的天空总是灰蒙蒙的，像是蒙了一层永远擦不干净的塑料膜。他想起老家，想起那片干净得发蓝的天，想起小时候跟着爷爷在田里干活的日子。那个时候的天亮得特别早，四五点钟东边就泛白了，整个村子都笼罩在一层薄薄的雾气里，空气里飘着泥土和青草的味道。\n\n　　「明远，你打算怎么办？」老黄问。\n\n　　周明远摇了摇头。他不知道。他兜里只剩下四百块钱，房租还差三天到期，手机已经欠费停机了。老婆林秀兰在老家带女儿，每个月都等着他寄钱回去。女儿小雨今年六岁，刚上小学一年级，前两天还打电话说学校要交二百块钱的书本费。\n\n　　想到女儿，他的胸口就发闷。\n\n　　小雨出生的时候他不在家，在工地赶工期。老婆打电话来的时候，他正在楼顶上绑钢筋，手机差点从二十楼掉下去。那天他请了一天假，坐大巴车回了老家，到医院的时候女儿已经抱在护士怀里了，皱巴巴的一小团，闭着眼睛，像一只小猫。他第一次抱女儿的时候手都是抖的，生怕力气太大弄疼了她。\n\n　　后来日子就这么一天天地过着。他在城里打工挣钱，老婆在家带孩子种地。每个月发工资那天是他最开心的时候，他会去镇上的小饭馆给老婆转钱，然后给女儿打一个电话，听她说「爸爸我想你了」。那四个字是他在这个城市里唯一的暖意。\n\n　　可是现在，连这份暖意都快要维持不下去了。\n\n　　老黄见他半天不说话，叹了口气，从裤兜里掏出一包皱巴巴的红塔山，抽出一根递给他：「你年轻，有手艺，不像我，干不动了。大不了换个工地接着干，天无绝人之路。」\n\n　　周明远接过烟，没点，捏在手里转了两圈。他忽然想起一件事——半个月前，有个戴眼镜的年轻人来过工地，说是什么网络小说平台的外包编辑，问他有没有兴趣写点东西。当时他觉得这事特别不靠谱，一个初中没毕业的农民工，能写什么小说？那人留了一张名片，他随手塞在工棚的工具箱里，后来就忘了。\n\n　　「老黄，你觉得……人这一辈子，是不是就该认命？」\n\n　　老黄愣了一下，然后乐了：「你小子今天怎么了？受刺激了？」\n\n　　周明远没笑。他看着老黄那张布满皱纹的脸，忽然觉得很害怕。他在老黄身上看到了二十年后的自己——背驼了，头发白了，干了一辈子，最后连工钱都拿不到，蹲在工地门口等天亮。\n\n　　他不要这样的人生。\n\n　　天彻底黑了。工地上的灯亮起来，惨白的光照在水泥地上，周围的城中村亮起了星星点点的灯火。远处传来广场舞的音乐声，还有小贩的叫卖声，和这个世界最底层的热闹搅在一起，像一锅煮烂了的面条。\n\n　　周明远站起来，拍了拍裤子上的灰。他做了一个决定。\n\n　　「老黄，我先走了。」\n\n　　「去哪儿？」\n\n　　「回去找找那张名片。」\n\n　　老黄没听懂，但也没多问。他看着周明远走出工地的背影，觉得这个年轻人今天哪里不一样了。平时的周明远走路总是低着头，像被什么东西压着，但今天他抬起了头，虽然背还是挺不直，但眼神里多了一点什么。\n\n　　周明远穿过城中村狭窄的巷子，绕过满地乱爬的电线，回到了自己租的那间十二平米的隔断房。屋里只有一张床、一张桌子和一把塑料凳。桌上摆着一台七年前买的二手笔记本电脑，是他在跳蚤市场花三百块钱淘来的，平时只用来看电影，键盘上的字母都已经磨掉了。\n\n　　他翻开工棚带回来的工具箱，在最底层的夹层里找到了那张名片。名片很普通，白底黑字，印着一个名字和一个手机号。\n\n　　陈哲，番茄小说内容编辑。\n\n　　周明远盯着这张名片看了很久，久到窗外的汽车喇叭声都变得模糊了起来。然后他打开那台笔记本电脑，等了整整两分钟才等到开机，打开了一个空白文档。\n\n　　光标一闪一闪的，像一颗心在跳。\n\n　　他犹豫了很久，在键盘上敲下了第一行字。\n\n　　他的手很粗糙，指节因为常年抬钢筋而变了形，敲键盘的样子看起来很笨拙。但他一个字一个字地敲着，越敲越快，像是有什么东西堵在胸口太久了，终于找到了一个出口。\n\n　　他写的不是别人的故事。\n\n　　他写的是自己。\n\n　　写那个十二年前从山村里走出来、以为能在城里改变命运的年轻人。写那些在工地上流过的汗水和眼泪，写那些深夜躺在硬板床上辗转反侧的日子，写那些给女儿打电话时拼命忍住不哭的时刻。\n\n　　他写得很慢，写得很笨，但他写得很认真。\n\n　　因为这是他这辈子第一次觉得，自己的人生，或许还有另一种可能。\n\n　　窗外，城市的夜晚依然喧嚣。而在这间十二平米的隔断房里，一个从来没有写过任何东西的农民工，在他那台老旧的笔记本电脑上，开始了他人生中的第一章。\n\n　　天总会亮的。\n\n　　只是，要等得够久。"
	req := c1_publisher.PublishRequest{
		TaskID: "task_demo_001",
		Products: map[string]c1_publisher.ProductContent{
			"fanqie": {
				Text:          chapterContent,
				NovelName:     "天亮的时候",
				VolumeName:    "第一卷",
				ChapterNumber: 1,
				Title:         "第一章 天黑之前",
			},
		},
		Accounts: []c1_publisher.AccountRef{
			{AccountID: acc3.AccountID, UID: "user_2", Platform: "fanqie"},
		},
		TraceID: "trace_demo_001",
	}

	// 5. 发布
	resp, err := publisher.Publish(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "发布失败: %v\n", err)
		os.Exit(1)
	}

	// 6. 查看结果
	fmt.Printf("\n===== 发布结果 =====\n")
	fmt.Printf("任务: %s\n", resp.TaskID)
	fmt.Printf("总计: %d | 成功: %d | 失败: %d\n",
		resp.Summary.Total, resp.Summary.Succeeded, resp.Summary.Failed)

	fmt.Printf("\n明细:\n")
	for _, r := range resp.Results {
		icon := "✅"
		if r.Status == "fail" {
			icon = "❌"
		}
		fmt.Printf("  %s %s | %s | %s | post_id=%s | err=%s\n",
			icon, r.Platform, r.AccountID, r.Status, r.PostID, r.ErrorCode)
	}

	// 7. 查看存储记录
	records, _ := publisher.GetStore().FindByTaskID(ctx, "task_demo_001")
	fmt.Printf("\n数据库记录数: %d\n", len(records))

	// 8. 查看 Mock Adapter 调用统计
	fmt.Printf("\n调用统计:\n")
	fmt.Printf("  番茄小说 Adapter 调用次数: %d\n", fanqieAdapter.GetCallCount())
}

func newMockA1Server(vault *a1_account.MockSecretVault) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/account/credentials" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var reqBody struct {
			AccountID string `json:"account_id"`
			UID       string `json:"uid"`
			Caller    string `json:"caller"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(c1_publisher.A1Error{Code: "INVALID_INPUT", Message: err.Error()})
			return
		}
		resp, err := vault.GetCredentials(r.Context(), a1_account.GetCredentialsRequest{
			AccountID: reqBody.AccountID,
			UID:       reqBody.UID,
			Caller:    reqBody.Caller,
		})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(c1_publisher.A1Error{Code: "UNKNOWN", Message: err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}
