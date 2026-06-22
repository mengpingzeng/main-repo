# ============================================================
#  Stage 1: Build — 编译所有 Go 模块 + 构建 Next.js 前端
# ============================================================
FROM ubuntu:24.04 AS build

ENV DEBIAN_FRONTEND=noninteractive \
    PATH="/usr/local/go/bin:${PATH}"

# 安装构建依赖
RUN apt-get update -qq && apt-get install -y -qq \
    curl ca-certificates git xz-utils build-essential && \
    rm -rf /var/lib/apt/lists/*

# 安装 Go 1.25（满足各模块 go 1.25.0 要求）
ENV GO_VERSION=1.25.2
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
    | tar -C /usr/local -xz

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.org

# 安装 Node.js 22 LTS
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt-get install -y -qq nodejs && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 复制源码
COPY . .

# 修正 Go module replace 路径（go.mod 中写死了 /home/main-repo 绝对路径）
RUN find . -name "go.mod" -print0 | xargs -0 -I{} sh -c \
    'q="{}"; d="$(dirname "$q")"; \
     sed -i "s|=> /home/main-repo/|=> /app/|g" "$q"'

# 下载 Go 模块依赖
RUN find . -name "go.mod" -print0 | xargs -0 -I{} sh -c \
    'd="$(dirname "{}")"; echo ">>> $d"; cd "$d" && go mod download 2>&1 || true'

# 编译所有 Go 二进制文件
RUN set -e; \
    echo ">>> Build skill_registry"     && cd L1_skills_register       && go build -o skill_registry . ; \
    echo ">>> Build ai_provider"        && cd /app/L1_AI_Provider       && go build -o ai_provider . ; \
    echo ">>> Build session_manager"    && cd /app/L2_conversion_manager && go build -o session_manager . ; \
    echo ">>> Build a1_server"          && cd /app/L0_AI_Account_Secret_Vault && go build -o a1_server ./cmd/a1_server/ ; \
    echo ">>> Build workflow_engine"    && cd /app/L2_AI_Workflow_Engine && go build -o workflow_engine ./cmd/workflow_engine/ ; \
    echo ">>> Build c2_dashboard"       && cd /app/L1_AI_Dashboard      && go build -o c2_dashboard ./cmd/c2_dashboard/ ; \
    echo ">>> Build scheduler"          && cd /app/L2_AI_Interval        && go build -o scheduler ./cmd/scheduler/ ; \
    echo ">>> Build bff-server"         && cd /app/L3_AI_BFF            && go build -o bff-server . ; \
    echo ">>> Build novelcover_pure"    && cd /app/L1_novel_cover_png    && go build -o novelcover_pure .

# 构建前端（生产模式）
RUN cd /app/Front_design && \
    echo ">>> npm install" && npm install && \
    echo ">>> npm run build" && npm run build

# ============================================================
#  Stage 2: Runtime — 生产运行环境
# ============================================================
FROM ubuntu:24.04

ARG DEEPSEEK_API_KEY

RUN echo "DEEPSEEK_API_KEY = ${DEEPSEEK_API_KEY}"

ENV DEBIAN_FRONTEND=noninteractive \
    RUN_USER=root \
    # ---------- 后端端口 ----------
    PORT=8088 \
    # ---------- 前端端口 ----------
    FE_PORT=3000 \
    # ---------- API_KEY
    DEEPSEEK_API_KEY=${DEEPSEEK_API_KEY} \
    # ---------- 数据库连接 ----------
    MYSQL_HOST=127.0.0.1 \
    MYSQL_PORT=3306 \
    MYSQL_ROOT_USER=root \
    MYSQL_ROOT_PASSWORD=claw123 \
    A1_DB_DSN="xlongxia:Xlongxia_123@tcp(127.0.0.1:3306)/xlongxia?parseTime=true" \
    A1_DB_PASSWORD="Xlongxia_123" \
    C2_DB_DSN="xlongxia:Xlongxia_123@tcp(127.0.0.1:3306)/xlongxia?parseTime=true" \
    WF_DB_DSN="xlongxia:Xlongxia_123@tcp(127.0.0.1:3306)/xlongxia?parseTime=true" \
    DB_DSN="root:claw123@tcp(127.0.0.1:3306)/claw_studios?parseTime=true&charset=utf8mb4" \
    # ---------- 加密/认证 ----------
    A1_ENCRYPTION_KEY="eLvMeGfepGpOUw280t7dvJTf+dkVAWn5B5dLOA4rMjk=" \
    A1_MOCK_ENCRYPTION_KEY="eLvMeGfepGpOUw280t7dvJTf+dkVAWn5B5dLOA4rMjk=" \
    A1_JWT_SECRET="not-default-secret-change-me" \
    JWT_SECRET="not-default-secret-change-me" \
    # ---------- 存储路径 ----------
    A4_STORAGE_DIR="/tmp/sm_demo" \
    STOPPED_TASKS_FILE="/tmp/sm_demo/stopped_tasks.json" \
    # ---------- 服务间通信（容器内均为 localhost） ----------
    SESSION_MGR_URL="http://localhost:18080" \
    WORKFLOW_URL="http://localhost:9100" \
    C2_DASHBOARD_URL="http://localhost:8083" \
    A1_ACCOUNT_URL="http://localhost:8084" \
    SKILL_REGISTRY_URL="http://localhost:18090" \
    AI_MODEL_URL="http://localhost:18180" \
    A1_BASE_URL="http://localhost:8084"

# 安装运行时依赖
RUN apt-get update -qq && apt-get install -y -qq \
    mysql-server mysql-client \
    curl ca-certificates git psmisc xz-utils \
    python3 python3-pip && \
    rm -rf /var/lib/apt/lists/*

# 安装 Node.js 22 LTS
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt-get install -y -qq nodejs && \
    rm -rf /var/lib/apt/lists/*

# 安装 opencode
RUN npm config set registry https://registry.npmmirror.com && \
    npm install -g opencode-ai

WORKDIR /app

# ---------- 复制二进制文件 ----------
COPY --from=build /app/L1_skills_register/skill_registry           /app/L1_skills_register/skill_registry
COPY --from=build /app/L1_AI_Provider/ai_provider                   /app/L1_AI_Provider/ai_provider
COPY --from=build /app/L2_conversion_manager/session_manager        /app/L2_conversion_manager/session_manager
COPY --from=build /app/L0_AI_Account_Secret_Vault/a1_server         /app/L0_AI_Account_Secret_Vault/a1_server
COPY --from=build /app/L2_AI_Workflow_Engine/workflow_engine        /app/L2_AI_Workflow_Engine/workflow_engine
COPY --from=build /app/L1_AI_Dashboard/c2_dashboard                 /app/L1_AI_Dashboard/c2_dashboard
COPY --from=build /app/L2_AI_Interval/scheduler                     /app/L2_AI_Interval/scheduler
COPY --from=build /app/L3_AI_BFF/bff-server                         /app/L3_AI_BFF/bff-server
COPY --from=build /app/L3_AI_BFF/config                            /app/L3_AI_BFF/config
COPY --from=build /app/L1_novel_cover_png/novelcover_pure           /app/L1_novel_cover_png/novelcover_pure

# ---------- 复制前端构建产物 ----------
COPY --from=build /app/Front_design/.next           /app/Front_design/.next
COPY --from=build /app/Front_design/node_modules     /app/Front_design/node_modules
COPY --from=build /app/Front_design/package.json     /app/Front_design/package.json
COPY --from=build /app/Front_design/next.config.ts   /app/Front_design/next.config.ts
COPY --from=build /app/Front_design/next.dev-origins.ts   /app/Front_design/next.dev-origins.ts
# COPY --from=build /app/Front_design/.env.local       /app/Front_design/.env.local
COPY --from=build /app/Front_design/public           /app/Front_design/public

# ---------- 复制静态资源 ----------
COPY --from=build /app/L1_novel_cover_png/fonts      /app/L1_novel_cover_png/fonts
COPY --from=build /app/L1_skills_register/fixtures   /app/L1_skills_register/fixtures

# ---------- 复制数据库 Schema ----------
COPY --from=build /app/schema_xlongxia.sql       /app/schema_xlongxia.sql
COPY --from=build /app/schema_claw_studios.sql   /app/schema_claw_studios.sql
COPY --from=build /app/migrations                /app/migrations

# ---------- 复制配置文件模板 ----------
COPY docker/L1_AI_Provider.keys.json             /app/L1_AI_Provider/config/keys.json
COPY docker/L1_novel_skill.config.json           /app/L1_novel_skill/config.json

# ---------- 复制启动脚本 ----------
COPY docker/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# ============================================================
# 持久化目录定义 (方便做 volume 映射)
# ============================================================
#   /tmp/sm_demo          — 小说输出目录 (openconfig.json, skills, stopped_tasks.json, 生成的小说章节)
#   /var/lib/mysql        — MySQL 数据目录 (用户数据、发布任务、账号凭证 等所有业务表)
#   /tmp/logs             — 日志目录

# ---------- 暴露端口 ----------
# 8088: BFF Gateway — 后端统一 API 入口
EXPOSE 8088
# 3000: Frontend — Next.js 前端 UI
EXPOSE 3000

# ---------- 健康检查 ----------
HEALTHCHECK --interval=30s --timeout=5s --start-period=120s --retries=3 \
    CMD curl -fsS http://localhost:8088/healthz || exit 1

ENTRYPOINT ["docker-entrypoint.sh"]
