#!/usr/bin/env node
/**
 * mcp_server_fanqie.js — 番茄小说 MCP Server
 *
 * 符合 MCP（Model Context Protocol）标准，使用 stdio 传输。
 * 提供 publish_chapter 工具，调用 Puppeteer 脚本自动发布章节。
 *
 * 运行环境：Linux 服务器，Node.js ≥ 18
 * 依赖：无需额外 npm 包，纯 Node.js 标准库实现。
 *
 * 使用方式：
 *   1. 确保 scripts/publish_fanqie.js 存在且安装了 puppeteer
 *   2. 设置环境变量 FANQIE_COOKIE
 *   3. 在 MCP 客户端配置中指向本文件：
 *      {
 *        "mcpServers": {
 *          "tomato-novel": {
 *            "command": "node",
 *            "args": ["/path/to/mcp_server_fanqie.js"],
 *            "env": { "FANQIE_COOKIE": "你的cookie" }
 *          }
 *        }
 *      }
 */

'use strict';

const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');
const readline = require('readline');

// ======================== 配置 ========================

// publish_fanqie.js 与 mcp_server_fanqie.js 在同一目录
const PUBLISH_SCRIPT = path.join(__dirname, 'publish_fanqie.js');

// 执行超时时间（毫秒）
const EXEC_TIMEOUT_MS = 90000;

// 强制 kill 前等待时间（毫秒）
const KILL_GRACE_MS = 2000;

// MCP 服务器信息
const SERVER_INFO = {
    name: 'tomato-novel-mcp',
    version: '1.0.0',
};

// ======================== 日志（仅 stderr） ========================

/** 打印日志到 stderr，不干扰 stdout 的 JSON-RPC 通信 */
function log(level, msg, data) {
    const entry = { time: new Date().toISOString(), level, msg, ...(data ? { data } : {}) };
    process.stderr.write(JSON.stringify(entry) + '\n');
}

// ======================== 启动前校验 ========================

/**
 * 检查 PUBLISH_SCRIPT 是否存在。
 * 不存在则输出错误日志并立即退出，不让 MCP 客户端无意义等待。
 */
function checkPrerequisites() {
    if (!fs.existsSync(PUBLISH_SCRIPT)) {
        log('error', 'PUBLISH_SCRIPT not found', { path: PUBLISH_SCRIPT });
        process.stderr.write('FATAL: publish_fanqie.js not found at ' + PUBLISH_SCRIPT + '\n');
        process.stderr.write('Please ensure the Puppeteer script exists and "npm install puppeteer" has been run.\n');
        process.exit(1);
    }
    log('info', 'PUBLISH_SCRIPT verified', { path: PUBLISH_SCRIPT });
}

// ======================== JSON-RPC ========================

/**
 * 发送 JSON-RPC 响应到 stdout。
 * MCP 协议要求每行一个完整的 JSON 对象。
 */
function sendResponse(id, result) {
    const response = { jsonrpc: '2.0', id, result };
    process.stdout.write(JSON.stringify(response) + '\n');
}

/** 发送 JSON-RPC 错误响应 */
function sendError(id, code, message, data) {
    const response = {
        jsonrpc: '2.0',
        id,
        error: { code, message, ...(data ? { data } : {}) },
    };
    process.stdout.write(JSON.stringify(response) + '\n');
}

/** 发送 JSON-RPC 通知（无 id，不需要响应） */
function sendNotification(method, params) {
    const notification = { jsonrpc: '2.0', method, params };
    process.stdout.write(JSON.stringify(notification) + '\n');
}

// ======================== MCP 方法处理器 ========================

/**
 * 处理 initialize 请求。
 * 返回服务器能力声明，告诉客户端我们提供 tools。
 */
function handleInitialize(id, params) {
    log('info', 'initialize', { clientName: params?.clientInfo?.name });
    sendResponse(id, {
        protocolVersion: '2024-11-05',
        capabilities: {
            tools: {}, // 声明支持 tools 能力
        },
        serverInfo: SERVER_INFO,
    });
}

/**
 * 处理 tools/list 请求。
 * 返回已注册的工具列表（目前只有一个 publish_chapter）。
 */
function handleToolsList(id) {
    sendResponse(id, {
        tools: [
            {
                name: 'publish_chapter',
                description: '发布一章小说到番茄小说平台（作家后台自动操作）。通过浏览器自动化登录、创建/查找作品、填写章节内容并发布。',
                inputSchema: {
                    type: 'object',
                    properties: {
                        title: {
                            type: 'string',
                            description: '章节标题，如"第一章 星辰坠落"',
                        },
                        content: {
                            type: 'string',
                            description: '章节正文，须 ≥ 1000 字',
                        },
                        novelName: {
                            type: 'string',
                            description: '作品名，如"剑破苍穹"',
                        },
                        volumeName: {
                            type: 'string',
                            description: '分卷名，默认"第一卷"',
                            default: '第一卷',
                        },
                        chapterNumber: {
                            type: 'number',
                            description: '章节序号，默认 1',
                            default: 1,
                        },
                    },
                    required: ['title', 'content', 'novelName'],
                },
            },
        ],
    });
}

/**
 * 处理 tools/call 请求。
 * 根据 name 路由到具体工具实现。
 */
async function handleToolsCall(id, params) {
    const { name, arguments: args } = params || {};

    if (name === 'publish_chapter') {
        await handlePublishChapter(id, args);
    } else {
        sendError(id, -32601, '未知工具: ' + name);
    }
}

/**
 * 执行 publish_chapter 工具。
 * 调用 Puppeteer 脚本并将结果返回给 MCP 客户端。
 *
 * 入参 JSON 经 base64 编码后传给子进程，避免换行/引号等特殊字符
 * 被 shell 截断或转义。
 */
async function handlePublishChapter(id, args) {
    // ---- 参数校验 ----
    if (!args || !args.title || !args.content || !args.novelName) {
        sendResponse(id, {
            content: [{
                type: 'text',
                text: '参数错误：title、content、novelName 为必填字段',
            }],
            isError: true,
        });
        return;
    }

    // ---- 拼装 JSON 入参，base64 编码传递给子进程 ----
    const input = {
        title: args.title,
        content: args.content,
        novelName: args.novelName,
        volumeName: args.volumeName || '第一卷',
        chapterNumber: args.chapterNumber || 1,
    };
    const inputJson = JSON.stringify(input);
    // base64 编码：规避 JSON 中的换行符、引号、Unicode 等在命令行传递时被截断
    const inputBase64 = Buffer.from(inputJson, 'utf-8').toString('base64');

    log('info', 'publish_chapter called', {
        title: input.title,
        novelName: input.novelName,
        volumeName: input.volumeName,
        chapterNumber: input.chapterNumber,
        contentLen: input.content.length,
        base64Len: inputBase64.length,
    });

    // ---- 检查 FANQIE_COOKIE ----
    const cookie = process.env.FANQIE_COOKIE;
    if (!cookie || cookie.trim() === '') {
        sendResponse(id, {
            content: [{
                type: 'text',
                text: '发布失败：FANQIE_COOKIE 环境变量未设置。请在 MCP 客户端配置中设置 FANQIE_COOKIE。',
            }],
            isError: true,
        });
        return;
    }

    // ---- 执行 Puppeteer 脚本 ----
    try {
        const result = await runPuppeteerScript(inputBase64, cookie);

        if (result.success) {
            sendResponse(id, {
                content: [{
                    type: 'text',
                    text: '发布成功，章节ID：' + result.postId,
                }],
            });
        } else {
            sendResponse(id, {
                content: [{
                    type: 'text',
                    text: '发布失败：' + (result.error || '未知错误'),
                }],
                isError: true,
            });
        }
    } catch (err) {
        log('error', 'puppeteer script error', { error: err.message });
        sendResponse(id, {
            content: [{
                type: 'text',
                text: '发布失败：脚本执行异常 — ' + err.message,
            }],
            isError: true,
        });
    }
}

/**
 * 启动子进程执行 publish_fanqie.js。
 *
 * 参数经 base64 编码后传入，子脚本端自动识别并解码。
 * stdin 通过管道写入 base64 字符串（而非命令行参数），
 * 彻底消除 shell 转义和参数长度限制问题。
 *
 * @param {string} inputBase64 - base64 编码的 JSON 入参
 * @param {string} cookie - 番茄小说 Cookie
 * @returns {Promise<{success: boolean, postId?: string, error?: string}>}
 */
function runPuppeteerScript(inputBase64, cookie) {
    return new Promise((resolve, reject) => {
        // 使用 spawn，通过 stdin 管道传入 base64 数据（而非命令行参数）
        // 子脚本从 stdin 读取 base64 → 解码 → 解析 JSON
        const child = spawn('node', [PUBLISH_SCRIPT, '--base64'], {
            env: {
                ...process.env,
                FANQIE_COOKIE: cookie,
            },
            stdio: ['pipe', 'pipe', 'pipe'], // stdin 改为 pipe，用于传入数据
        });

        let stdout = '';
        let stderr = '';

        child.stdout.on('data', (chunk) => {
            stdout += chunk.toString();
        });

        child.stderr.on('data', (chunk) => {
            stderr += chunk.toString();
            // 转发脚本的 stderr 到本服务的 stderr（便于调试）
            process.stderr.write(chunk);
        });

        // 将 base64 数据写入子进程 stdin 后关闭，触发子脚本读取
        child.stdin.write(inputBase64);
        child.stdin.end();

        // 超时处理
        const timer = setTimeout(() => {
            child.kill('SIGTERM');
            // SIGTERM 后等待 KILL_GRACE_MS 再强制 SIGKILL
            setTimeout(() => {
                try { child.kill('SIGKILL'); } catch (_) { /* 进程可能已退出 */ }
            }, KILL_GRACE_MS);
            reject(new Error('脚本执行超时（' + (EXEC_TIMEOUT_MS / 1000) + ' 秒）'));
        }, EXEC_TIMEOUT_MS);

        child.on('close', (code) => {
            clearTimeout(timer);

            log('info', 'puppeteer script exited', { exitCode: code, stdoutLen: stdout.length });

            const trimmed = stdout.trim();
            if (!trimmed) {
                reject(new Error('脚本未输出任何内容'));
                return;
            }

            try {
                const result = JSON.parse(trimmed);
                resolve(result);
            } catch (parseErr) {
                log('error', 'parse script output failed', {
                    error: parseErr.message,
                    raw: trimmed.substring(0, 500),
                });
                reject(new Error('脚本输出解析失败: ' + parseErr.message));
            }
        });

        child.on('error', (err) => {
            clearTimeout(timer);
            log('error', 'failed to start puppeteer script', { error: err.message });
            reject(new Error('无法启动 Node.js 进程: ' + err.message));
        });
    });
}

// ======================== 主循环：stdio JSON-RPC ========================

/**
 * 从 stdin 逐行读取 JSON-RPC 请求，处理并写回 stdout。
 * stderr 仅用于日志，不影响协议通信。
 */
function main() {
    // 启动前校验
    checkPrerequisites();

    log('info', 'MCP server starting', { script: PUBLISH_SCRIPT });

    // 通知客户端服务器已就绪（某些 MCP 客户端需要此通知）
    process.nextTick(() => {
        log('info', 'MCP server ready');
    });

    const rl = readline.createInterface({
        input: process.stdin,
        output: process.stdout,
        terminal: false,
    });

    rl.on('line', async (line) => {
        // 跳过空行
        if (!line.trim()) {
            return;
        }

        let request;
        try {
            request = JSON.parse(line);
        } catch (err) {
            log('warn', 'invalid JSON received', { raw: line.substring(0, 200) });
            sendError(null, -32700, 'Parse error: ' + err.message);
            return;
        }

        // 验证 JSON-RPC 基本格式
        if (!request.jsonrpc || request.jsonrpc !== '2.0') {
            sendError(request.id || null, -32600, 'Invalid Request: jsonrpc must be "2.0"');
            return;
        }

        const { id, method, params } = request;

        try {
            switch (method) {
                case 'initialize':
                    handleInitialize(id, params);
                    break;

                case 'notifications/initialized':
                    // 客户端确认初始化完成：记录日志并发送 initialized 通知
                    log('info', 'client initialized');
                    sendNotification('initialized', {});
                    break;

                case 'tools/list':
                    handleToolsList(id);
                    break;

                case 'tools/call':
                    await handleToolsCall(id, params);
                    break;

                case 'ping':
                    // 心跳检测（某些 MCP 客户端会发 ping）
                    sendResponse(id, {});
                    break;

                default:
                    log('warn', 'unknown method', { method });
                    sendError(id, -32601, 'Method not found: ' + method);
            }
        } catch (err) {
            log('error', 'handler error', { method, error: err.message, stack: err.stack });
            sendError(id, -32603, 'Internal error: ' + err.message);
        }
    });

    rl.on('close', () => {
        log('info', 'stdin closed, MCP server shutting down');
        process.exit(0);
    });

    // 优雅退出
    process.on('SIGTERM', () => {
        log('info', 'SIGTERM received, shutting down');
        process.exit(0);
    });
    process.on('SIGINT', () => {
        log('info', 'SIGINT received, shutting down');
        process.exit(0);
    });
}

// ======================== 启动 ========================
main();
