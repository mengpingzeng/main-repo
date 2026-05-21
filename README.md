# 主仓库 - 子模块管理

本项目采用 Git Subtree 管理各个独立子模块。所有子模块均有自己的公开 GitHub 仓库。

## 子模块列表
| 模块 | 独立仓库 |
|------|----------|
| fanqie-account-manager | https://github.com/mengpingzeng/fanqie-account-manager |
| Front_design | https://github.com/mengpingzeng/Front_design |
| frontend | https://github.com/mengpingzeng/frontend |
| L0_AI_Account_Secret_Vault | https://github.com/mengpingzeng/L0_AI_Account_Secret_Vault |
| L1_AI_Dashboard | https://github.com/mengpingzeng/L1_AI_Dashboard |
| L1_AI_Doc_Hub | https://github.com/mengpingzeng/L1_AI_Doc_Hub |
| L1_AI_Provider | https://github.com/mengpingzeng/L1_AI_Provider |
| L1_AI_Releaser | https://github.com/mengpingzeng/L1_AI_Releaser |
| L1_opencode | https://github.com/mengpingzeng/L1_opencode |
| L1_skills_register | https://github.com/mengpingzeng/L1_skills_register |
| L2_AI_Interval | https://github.com/mengpingzeng/L2_AI_Interval |
| L2_AI_Workflow_Engine | https://github.com/mengpingzeng/L2_AI_Workflow_Engine |
| L2_conversion_manager | https://github.com/mengpingzeng/L2_conversion_manager |
| L3_AI_BFF | https://github.com/mengpingzeng/L3_AI_BFF |
| migrations | https://github.com/mengpingzeng/migrations |

## 独立更新某个子模块
1. 进入该子模块的原始目录（例如 `/home/claw_studios/code/service-a`）
2. 修改代码，提交并推送：
   ```bash
   git add -A && git commit -m "your message"
   git push origin main
   ```
3. 回到主仓库，拉取该子模块的更新：
   ```bash
   cd /home/claw_studios/code/main-repo
   git fetch git@github.com:mengpingzeng/service-a.git main
   git read-tree --prefix=service-a/ -u FETCH_HEAD
   git commit -m "Update service-a subtree"
   git push origin main
   ```

## 在主仓库直接修改并回传子模块
如果直接在主仓库的 service-a/ 中修改了代码：
```bash
git add service-a/ && git commit -m "update service-a"
# 切换到子模块仓库推送
cd /home/claw_studios/code/service-a
git pull ../main-repo main --allow-unrelated-histories
git push origin main
```

## 克隆主仓库（包含所有子模块代码）
```bash
git clone git@github.com:mengpingzeng/main-repo.git
```
无需额外步骤，所有代码已直接包含在主仓库中。
