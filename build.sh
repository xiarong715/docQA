#!/bin/bash

# 编译脚本 - 启用 CGO
CGO_ENABLED=1 go mod tidy
CGO_ENABLED=1 go build -o rag_qa.exe
echo "✅ 编译完成！"
