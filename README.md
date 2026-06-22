# Claw Studios — 全模块工程

所有代码已包含在主仓库中，`git clone` 后完成前置配置即可一键启动。

---

## 启动前必须手动完成

以下配置脚本无法自动化，请在首次启动前完成：

### 1. 安装 Go

Go 各发行版安装方式差异太大，脚本不自动安装。

```bash
# 检查是否已安装
go version  # 需要 >= 1.21

# 阿里云 Linux / CentOS
yum install -y golang

# 或通过官方二进制安装（推荐 1.24+）
wget https://go.dev/dl/go1.24.11.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.11.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

### 2. 设置 DeepSeek API Key（必须）

此为 opencode AI 写作引擎的唯一认证凭证，变量名不可更改。

```bash
export DEEPSEEK_API_KEY=sk-xxxxxxxxxxxxxxxx
```

> 获取: https://platform.deepseek.com → API Keys  
> 变量名 `DEEPSEEK_API_KEY` 是 opencode 的标准环境变量。

### 3. 创建 AI Provider 密钥文件

在启动脚本所在目录下创建 `L1_AI_Provider/config/keys.json`：

```json
{
  "deepseek": ["sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"]
}
```

> 支持多个 Key 轮转调用。

### 4. (可选) 创建混元生图密钥文件

如需小说封面自动生成，创建 `L1_novel_skill/config.json`：

```json
{
  "tencent_secret_id": "AKIDxxxxxxxxxxxxxxxx",
  "tencent_secret_key": "xxxxxxxxxxxxxxxxxxxx"
}
```

> 先到 https://buy.cloud.tencent.com/aiart 购买「腾讯混元生图极速版」  
> 再到 https://console.cloud.tencent.com/cam/capi 获取密钥  
> 再到 https://console.cloud.tencent.com/aiart 确认已开通 TextToImageLite

### 5. (可选) 安装 Python3 + Pillow

封面生成脚本需要。

```bash
yum install -y python3
pip3 install Pillow
```

---

## 启动服务

```bash
# 必须以 root 执行（脚本自动提权）
sudo bash start_all.sh
```

脚本自动完成：

| 步骤 | 说明 |
|------|------|
| 提权 | 非 root 自动 `sudo` 切换到 root |
| 安装 MySQL | 检测缺失则 `yum/apt install mysql-server`，配置 root 密码为 `claw123` |
| 安装 opencode | 检测缺失则 `npm install -g @anthropic/opencode`（含 Node.js） |
| 环境检测 | 检查 `DEEPSEEK_API_KEY`、`keys.json`、`config.json` 是否存在并提示 |
| 建库建表 | `schema_xlongxia.sql` + `schema_claw_studios.sql`（IF NOT EXISTS，幂等） |
| 编译 | 8 个 Go 后端 + Next.js 前端 |
| 启动 | 9 个服务顺序启动 + 健康检查 |

---

## 服务端口

| 端口 | 服务 | 说明 |
|------|------|------|
| 18080 | Session Manager | AI 会话管理 |
| 18090 | Skills Register | Skill 风格仓库 |
| 18180 | AI Provider | API Key 钱包 |
| 8083 | Dashboard | 发布看板 |
| 8084 | A1 Account Vault | 账号凭证加密 |
| 8088 | BFF Gateway | 前端统一入口 |
| 9100 | Workflow Engine | 发布工作流 |
| 9104 | Interval Scheduler | 定时调度 |
| 3000 | Frontend | Next.js 前端 |

入口: http://localhost:3000 / http://localhost:8088

---

## 自定义 MySQL 密码

如果你的 MySQL root 密码不是 `claw123`，通过环境变量覆盖：

```bash
export MYSQL_ROOT_PASSWORD=你的密码

# 如果两个业务库的密码也不同，一并设置
export A1_DB_DSN="xlongxia:业务密码@tcp(127.0.0.1:3306)/xlongxia?parseTime=true"
export DB_DSN="root:业务密码@tcp(127.0.0.1:3306)/claw_studios?parseTime=true&charset=utf8mb4"
export A1_DB_PASSWORD=业务密码

sudo bash start_all.sh
```

脚本 `setup_mysql_auth()` 会自动处理三种常见情况：
1. 新装 MySQL 无密码 → 自动设为 `claw123`
2. Ubuntu auth_socket 认证 → 自动改为密码认证
3. 已有密码 → 直接用

---

## 完整环境变量参考

以下变量均可通过 `export` 覆盖默认值：

### 必设
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DEEPSEEK_API_KEY` | (空) | DeepSeek API Key，不设 AI 无法工作 |

### MySQL 建库认证
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `MYSQL_ROOT_USER` | `root` | root 用户名 |
| `MYSQL_ROOT_PASSWORD` | `claw123` | root 密码 |
| `MYSQL_HOST` | `127.0.0.1` | 主机 |
| `MYSQL_PORT` | `3306` | 端口 |

### 业务库连接
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `A1_DB_DSN` | `xlongxia:Xlongxia_123@tcp(127.0.0.1:3306)/xlongxia?parseTime=true` | xlongxia 库 |
| `C2_DB_DSN` | 同 `A1_DB_DSN` | Dashboard 库 |
| `WF_DB_DSN` | 同 `A1_DB_DSN` | Workflow 库 |
| `DB_DSN` | `root:claw123@tcp(127.0.0.1:3306)/claw_studios?...` | claw_studios 库 |
| `A1_DB_USER` | `xlongxia` | 业务用户 |
| `A1_DB_PASSWORD` | `Xlongxia_123` | 业务密码 |

### 安全
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `A1_ENCRYPTION_KEY` | (内置) | 凭证加密密钥，生产环境更换 |
| `A1_JWT_SECRET` | `not-default-secret-change-me` | JWT 签名，生产环境更换 |
| `JWT_SECRET` | `not-default-secret-change-me` | JWT 签名（BFF 用） |

### 服务发现（一般无需改）
| 变量 | 默认值 |
|------|--------|
| `PORT` | `8088` |
| `SESSION_MGR_URL` | `http://localhost:18080` |
| `SKILL_REGISTRY_URL` | `http://localhost:18090` |
| `AI_MODEL_URL` | `http://localhost:18180` |
| `WORKFLOW_URL` | `http://localhost:9100` |
| `C2_DASHBOARD_URL` | `http://localhost:8083` |
| `A1_ACCOUNT_URL` | `http://localhost:8084` |
| `A1_BASE_URL` | `http://localhost:8084` |

### 运行用户
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `RUN_USER` | 自动检测 | 运行服务的系统用户 |

---

## 目录结构

| 目录 | 用途 |
|------|------|
| `Front_design/` | Next.js 前端 UI |
| `L0_AI_Account_Secret_Vault/` | 账号凭证加密保险库 |
| `L1_AI_Dashboard/` | 发布看板 |
| `L1_AI_Doc_Hub/` | 文档管理中心 |
| `L1_AI_Provider/` | API Key 钱包 |
| `L1_AI_Releaser/` | 内容发布器 |
| `L1_novel_skill/` | 灰度改写元技能 (AI Skill) |
| `L1_novel_cover_png/` | 封面文字渲染器 (纯 Go) |
| `L1_opencode/` | OpenCode 自定义 Skill |
| `L1_skills_register/` | Skill 注册中心 |
| `L2_AI_Interval/` | 定时调度器 |
| `L2_AI_Workflow_Engine/` | 发布工作流引擎 |
| `L2_conversion_manager/` | 会话管理器 |
| `L3_AI_BFF/` | BFF 网关 |
| `pkg/` | 共享库 |
| `migrations/` | 增量迁移 SQL |
| `fanqie-account-manager/` | Chrome 扩展 (番茄 Cookie 抓取) |
| `frontend/` | 前端元数据 |

### fanqie-account-manager 说明

Chrome 浏览器扩展，辅助抓取番茄小说 Cookie。安装: Chrome → `chrome://extensions/` → 开发者模式 → 加载 `extension/`

---

## 常见问题

### 远程访问

编辑 `Front_design/.env.local`，改 `localhost` 为服务器 IP:
```
NEXT_PUBLIC_API_BASE=http://服务器IP:8088
NEXT_PUBLIC_WS_BASE=ws://服务器IP:8088
```

### 阿里云安全组

需在安全组入方向放行: 3000, 8083, 8084, 8088, 9100, 9104, 18080, 18090, 18180

### Go 版本问题

```bash
# gvm 管理多版本
bash < <(curl -s -S -L https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer)
gvm install go1.24 && gvm use go1.24
```

### GOPROXY (国内加速)

```bash
export GOPROXY=https://goproxy.cn,direct
```

---

## Git Subtree

各子模块独立 GitHub 仓库:

| 模块 | 仓库 |
|------|------|
| fanqie-account-manager | github.com/mengpingzeng/fanqie-account-manager |
| Front_design | github.com/mengpingzeng/Front_design |
| L0_AI_Account_Secret_Vault | github.com/mengpingzeng/L0_AI_Account_Secret_Vault |
| L1_AI_Dashboard | github.com/mengpingzeng/L1_AI_Dashboard |
| L1_AI_Doc_Hub | github.com/mengpingzeng/L1_AI_Doc_Hub |
| L1_AI_Provider | github.com/mengpingzeng/L1_AI_Provider |
| L1_AI_Releaser | github.com/mengpingzeng/L1_AI_Releaser |
| L1_opencode | github.com/mengpingzeng/L1_opencode |
| L1_skills_register | github.com/mengpingzeng/L1_skills_register |
| L2_AI_Interval | github.com/mengpingzeng/L2_AI_Interval |
| L2_AI_Workflow_Engine | github.com/mengpingzeng/L2_AI_Workflow_Engine |
| L2_conversion_manager | github.com/mengpingzeng/L2_conversion_manager |
| L3_AI_BFF | github.com/mengpingzeng/L3_AI_BFF |
| migrations | github.com/mengpingzeng/migrations |
| frontend | github.com/mengpingzeng/frontend |
| L1_novel_skill / L1_novel_cover_png / pkg | 本地仓库（内置于本仓库） |

## 构建docker部署镜像

### 构建镜像

- 1.在构建镜像之前准备好以下文件：
  - docker/L1_AI_Provider.keys.json
  - docker/L1_novel_skill.config.json

- 2.执行
  ```
  python3 build_docker_image.py --api-key=<XXX>
  ```
  > 执行以后生成镜像：crpi-o1vsdbpms95lywf7.cn-shenzhen.personal.cr.aliyuncs.com/cszwf/zwf:<时间戳>
  
### 发布镜像

- 登录阿里云 Container Registry
```
docker login --username=lee_kenlao crpi-o1vsdbpms95lywf7.cn-shenzhen.personal.cr.aliyuncs.com
```

- 将镜像推送到Registry
```
docker tag [ImageId] crpi-o1vsdbpms95lywf7.cn-shenzhen.personal.cr.aliyuncs.com/cszwf/zwf:[镜像版本号]
docker push crpi-o1vsdbpms95lywf7.cn-shenzhen.personal.cr.aliyuncs.com/cszwf/zwf:[镜像版本号]
```

### 部署镜像

- 1.将`docker/docker-compose.yml`拷贝到部署环境中
- 2.从Registry中拉取镜像
  ```
  docker pull crpi-o1vsdbpms95lywf7.cn-shenzhen.personal.cr.aliyuncs.com/cszwf/zwf:[镜像版本号]
  ```
- 3.执行`docker compose up`

### 调试容器

- 查看容器
  ```
  docker ps -a
  ```
- 进入容器
  ```
  docker exec -it 容器名/容器ID bash
  ```