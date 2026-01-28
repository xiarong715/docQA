# 基于 Golang + LLM + RAG 的智能文档问答系统
https://www.doubao.com/thread/w93ab4baea9e3ebcd

# 启动 chroma
```bash
docker run -d -p 8000:8000 -v $(pwd)/chroma_data:/chroma/chroma chromadb/chroma:latest
```

# /etc/docker/daemon.json
{"registry-mirrors":["https://docker.m.daocloud.io","https://docker.1ms.run","https://dockerproxy.com"]}

```bash
systemctl restart docker
```

# 启动项目
```bash
go mod init rag_qa
go build
./rag_qa
```

# 加载文档
```bash
curl -X POST http://127.0.0.1:8080/load-doc -d "doc_path=test_doc.txt"
```

# 发起文档问答请求
```bash
curl -X POST http://127.0.0.1:8080/qa -d "question=Go语言的核心优势是什么？"
```