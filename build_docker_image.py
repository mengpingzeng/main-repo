#!/usr/bin/env python3
import os
import subprocess
from datetime import datetime
import sys
import argparse

def main():
    # 解析命令行参数
    parser = argparse.ArgumentParser(description="构建zwf镜像，传入DeepSeek API Key")
    parser.add_argument("--api-key", required=True, help="DEEPSEEK_API_KEY 值")
    args = parser.parse_args()
    deepseek_api_key = args.api_key

    # 1. 定义配置文件路径校验
    config_files = [
        "docker/L1_AI_Provider.keys.json",
        "docker/L1_novel_skill.config.json"
    ]

    # 2. 检查配置文件是否存在，缺失直接报错退出
    missing = []
    for fpath in config_files:
        if not os.path.isfile(fpath):
            missing.append(fpath)
    if missing:
        print("错误：以下配置文件缺失，请先创建：")
        for m in missing:
            print(f" - {m}")
        sys.exit(1)

    # 3. 生成时间标签 格式 YYYYMMDD_HHMMSS
    tag_time = datetime.now().strftime("%Y%m%d_%H%M%S")
    image_tag = f"crpi-o1vsdbpms95lywf7.cn-shenzhen.personal.cr.aliyuncs.com/cszwf/zwf:{tag_time}"

    # 4. 组装docker build命令，追加build-arg传递API KEY
    cmd = [
        "docker", "build",
        "-t", image_tag,
        "--build-arg", f"DEEPSEEK_API_KEY={deepseek_api_key}",
        "-f", "docker/Dockerfile",
        "."
    ]
    print(f"开始构建镜像，标签：{image_tag}")
    print(f"执行命令：{' '.join(cmd)}")

    # 5. 执行构建，构建失败抛出异常退出
    ret = subprocess.run(cmd)
    if ret.returncode != 0:
        print(f"构建失败，返回码：{ret.returncode}")
        sys.exit(ret.returncode)
    print(f"镜像构建完成：{image_tag}")

if __name__ == "__main__":
    main()
