#!/bin/bash

# 安装 CGO 依赖的脚本
# 用于 Ubuntu/Debian 系统

echo "正在安装 CGO 相关依赖..."

# 更新包列表
apt-get update

# 安装 GCC 和 CGO 必要的依赖
apt-get install -y build-essential gcc libc6-dev

echo "✅ CGO 依赖安装完成！"
echo ""
echo "现在可以运行以下命令编译项目："
echo "  go mod tidy"
echo "  go build"
