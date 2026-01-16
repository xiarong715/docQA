# 基于 Golang + LLM + RAG 的智能文档问答系统


# 启动 chroma
docker run -d -p 8000:8000 -v $(pwd)/chroma_data:/chroma/chroma chromadb/chroma:latest


https://www.doubao.com/thread/w93ab4baea9e3ebcd

# /etc/docker/daemon.json
{"registry-mirrors":["https://docker.m.daocloud.io","https://docker.1ms.run","https://dockerproxy.com"]}
