package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	chroma "github.com/amikos-tech/chroma-go/pkg/api/v2"
	chromaembeddings "github.com/amikos-tech/chroma-go/pkg/embeddings"
	chromaopenai "github.com/amikos-tech/chroma-go/pkg/embeddings/openai"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

// ========== 全局配置（修改为你的配置） ==========
const (
	// 1. 替换为你的OpenAI兼容API Key
	OpenAIAPIKey = "sk-xxxxxxxxxxxxxxxxxxxxxxxxx" // 测试时改为可用的key
	// 2. 替换为你的API地址（OpenAI官方：https://api.openai.com/v1；智谱：https://open.bigmodel.cn/api/paas/v4/；DeepSeek：https://api.deepseek.com/v1）
	OpenAIAPIBase = "https://dashscope.aliyuncs.com/compatible-mode/v1" //"https://api.openai.com/v1"
	// 3. 文本分块配置（最优值）
	ChunkSize    = 800 // 每个切片的字符数
	ChunkOverlap = 100 // 切片重叠字符数
	// 4. 检索配置
	TopK = 3 // 召回最相关的3个文档片段
)

// 全局客户端
var (
	openaiClient *openai.Client
	chromaClient chroma.Client
	collection   chroma.Collection // 向量库集合，存储文档向量
)

func init() {
	// 1. 初始化OpenAI客户端（兼容所有OpenAI接口的LLM）
	cfg := openai.DefaultConfig(OpenAIAPIKey)
	cfg.BaseURL = OpenAIAPIBase
	openaiClient = openai.NewClientWithConfig(cfg)

	// 2. 初始化ChromaDB客户端（连接到本地Chroma服务器，需先启动）
	var err error
	chromaClient, err = chroma.NewHTTPClient(chroma.WithBaseURL("http://172.17.0.1:8000"))
	if err != nil {
		panic(fmt.Sprintf("初始化向量库失败: %v", err))
	}

	// 3. 创建/获取向量库集合
	ctx := context.Background()
	// 创建 embedding 函数
	embeddingFunc, err := chromaopenai.NewOpenAIEmbeddingFunction(
		OpenAIAPIKey,
		chromaopenai.WithModel(chromaopenai.EmbeddingModel(openai.SmallEmbedding3)),
	)
	if err != nil {
		panic(fmt.Sprintf("创建 embedding 函数失败: %v", err))
	}
	// 先尝试获取已存在的集合，不存在则创建
	collection, err = chromaClient.GetOrCreateCollection(
		ctx,
		"doc_qa_collection",
		chroma.WithEmbeddingFunctionCreate(embeddingFunc),
	)
	if err != nil {
		panic(fmt.Sprintf("获取/创建集合失败: %v", err))
	}
	fmt.Println("✅ 初始化完成：LLM客户端 + 向量库")
}

// ========== 核心1：文本分块（切片）函数 ==========
func SplitText(text string, chunkSize int, chunkOverlap int) []string {
	var chunks []string
	text = strings.TrimSpace(text)
	if len(text) <= chunkSize {
		return []string{text}
	}

	// 按字符分块，带重叠窗口
	start := 0
	for start < len(text) {
		end := start + chunkSize
		if end > len(text) {
			end = len(text)
		}
		chunk := text[start:end]
		chunks = append(chunks, chunk)
		// 向前移动：块大小 - 重叠大小，保证语义连贯
		start += chunkSize - chunkOverlap
	}
	return chunks
}

// ========== 核心2：文本向量化函数 ==========
func GetEmbedding(text string) ([]float32, error) {
	req := openai.EmbeddingRequest{
		Input: []string{text},
		Model: "text-embedding-v1", //openai.QianwenEmbeddingV1, // 调用千问的文本向量化模型  // 也可用 text-embedding-ada-002，效果更好
	}
	resp, err := openaiClient.CreateEmbeddings(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("向量化失败: %v", err)
	}
	return resp.Data[0].Embedding, nil
}

// ========== 核心3：加载本地文档到向量库 ==========
func LoadDocToVectorDB(docPath string) error {
	// 读取本地文档（txt为例，可扩展pdf/docx）
	content, err := os.ReadFile(docPath)
	if err != nil {
		return fmt.Errorf("读取文档失败: %v", err)
	}
	text := string(content)

	// 1. 文本分块
	chunks := SplitText(text, ChunkSize, ChunkOverlap)
	fmt.Printf("📄 文档分块完成，共生成 %d 个切片\n", len(chunks))

	// 2. 遍历切片，向量化并入库
	for i, chunk := range chunks {
		embedding, err := GetEmbedding(chunk)
		if err != nil {
			fmt.Printf("切片 %d 向量化失败: %v\n", i, err)
			continue
		}
		// 向量入库：使用 WithEmbeddings 和 WithTexts
		emb := chromaembeddings.NewEmbeddingFromFloat32(embedding)
		err = collection.Add(context.Background(),
			chroma.WithIDs(chroma.DocumentID(fmt.Sprintf("doc_chunk_%d", i))),
			chroma.WithTexts(chunk),
			chroma.WithEmbeddings(emb),
		)
		if err != nil {
			fmt.Printf("切片 %d 入库失败: %v\n", i, err)
			continue
		}
	}
	fmt.Println("✅ 文档成功加载到向量库！")
	return nil
}

// ========== 核心4：RAG问答核心逻辑（检索+生成） ==========
func RAGQA(question string) (string, error) {
	// 第一步：用户问题向量化
	quesEmbedding, err := GetEmbedding(question)
	if err != nil {
		return "", fmt.Errorf("问题向量化失败: %v", err)
	}

	// 第二步：向量库相似度检索 - 召回TopK最相关的文档片段
	queryEmb := chromaembeddings.NewEmbeddingFromFloat32(quesEmbedding)
	queryResp, err := collection.Query(context.Background(),
		chroma.WithQueryEmbeddings(queryEmb),
		chroma.WithIncludeQuery(chroma.IncludeDocuments),
		chroma.WithNResults(TopK),
	)
	if err != nil {
		return "", fmt.Errorf("向量检索失败: %v", err)
	}
	// 拼接检索到的文档内容
	docs := queryResp.GetDocumentsGroups()[0]
	var docStrings []string
	for _, doc := range docs {
		docStrings = append(docStrings, doc.ContentString())
	}
	contextDocs := strings.Join(docStrings, "\n\n")
	fmt.Printf("🔍 检索到相关文档片段：\n%s\n", contextDocs)

	// 第三步：构建Prompt提示词（核心！决定LLM回答质量）
	prompt := fmt.Sprintf(`
你是一个专业的文档问答助手，你的回答必须严格基于以下提供的文档内容，不要编造任何信息。
如果文档中没有相关内容，请直接回答："文档中未找到相关信息"。
回答要求：简洁、准确、条理清晰，使用中文回答。

【参考文档内容】：
%s

【用户问题】：%s
`, contextDocs, question)

	// 第四步：调用LLM生成答案
	completionReq := openai.ChatCompletionRequest{
		Model: "qwen3-max-2026-01-23", //openai.GPT3Dot5Turbo, // 兼容智谱glm-4、deepseek-chat等
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.1, // 温度越低，回答越精准，无幻觉
		MaxTokens:   1024,
	}
	resp, err := openaiClient.CreateChatCompletion(context.Background(), completionReq)
	if err != nil {
		return "", fmt.Errorf("调用LLM失败: %v", err)
	}
	return resp.Choices[0].Message.Content, nil
}

// ========== API接口定义 ==========
func main() {
	r := gin.Default()

	// 1. 加载文档接口：POST /load-doc 传入文档路径
	r.POST("/load-doc", func(c *gin.Context) {
		docPath := c.PostForm("doc_path")
		if docPath == "" {
			c.JSON(400, gin.H{"code": 400, "msg": "文档路径不能为空"})
			return
		}
		err := LoadDocToVectorDB(docPath)
		if err != nil {
			c.JSON(500, gin.H{"code": 500, "msg": "加载失败", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "msg": "文档加载成功"})
	})

	// 2. 问答接口：POST /qa 传入用户问题
	r.POST("/qa", func(c *gin.Context) {
		question := c.PostForm("question")
		if question == "" {
			c.JSON(400, gin.H{"code": 400, "msg": "问题不能为空"})
			return
		}
		answer, err := RAGQA(question)
		if err != nil {
			c.JSON(500, gin.H{"code": 500, "msg": "问答失败", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "msg": "success", "answer": answer})
	})

	// 启动服务
	fmt.Println("🚀 智能文档问答服务启动成功：http://127.0.0.1:8080")
	_ = r.Run(":8080")
}
