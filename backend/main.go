package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/sashabaranov/go-openai"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 允许所有来源
		return true
	},
}

type ChatClient struct {
	openaiClient *openai.Client
	model        string
	messages     []openai.ChatCompletionMessage // 用于存储历史消息，实现多轮对话
}

func main() {
	_ = godotenv.Load()

	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_API_BASE")
	model := os.Getenv("OPENAI_API_MODEL")
	if apiKey == "" || baseURL == "" || model == "" {
		fmt.Println("检查环境变量设置")
		return
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL
	openaiClient := openai.NewClientWithConfig(config)

	endpoint := os.Getenv("OSS_ENDPOINT")
	accessKeyID := os.Getenv("OSS_ACCESS_KEY_ID")
	accessKeySecret := os.Getenv("OSS_ACCESS_KEY_SECRET")
	bucketName := os.Getenv("OSS_BUCKET")
	if endpoint == "" || accessKeyID == "" || accessKeySecret == "" || bucketName == "" {
		fmt.Println("检查环境变量设置")
		return
	}

	ossClient, err := NewOSSClient(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		fmt.Println(err)
		return
	}

	bucket, err := ossClient.CreateBucket(os.Getenv("OSS_BUCKET"))
	if err != nil {
		fmt.Println(err)
		return
	}

	cc := &ChatClient{
		openaiClient: openaiClient,
		model:        model,
		messages:     make([]openai.ChatCompletionMessage, 0),
	}

	r := gin.Default()

	// 允许跨域
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 上传文件
	r.POST("/upload", func(c *gin.Context) {
		file, err := c.FormFile("image")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// filename := fmt.Sprintf("uploads/%d_%s", time.Now().Unix(), filepath.Base(file.Filename))
		// err = c.SaveUploadedFile(file, filename)
		// if err != nil {
		//  c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		//  return
		// }

		fileContent, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer fileContent.Close()

		objectKey := fmt.Sprintf("uploads/%d_%s", time.Now().Unix(), filepath.Base(file.Filename))

		err = bucket.PutObject(objectKey, fileContent, oss.ObjectACL(oss.ACLPublicRead))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ossURL := fmt.Sprintf("https://%s.%s/%s", bucketName, endpoint, objectKey)

		c.JSON(http.StatusOK, gin.H{
			"errno": 0,
			"data":  ossURL,
		})
	})

	// 删除图片
	r.POST("/delete-image", func(c *gin.Context) {
		var req struct {
			Filename string `json:"filename"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 简单校验防止路径穿越
		if strings.Contains(req.Filename, "/") || strings.Contains(req.Filename, "\\") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "非法文件名"})
			return
		}

		// filePath := filepath.Join("uploads", req.Filename)
		// if err := os.Remove(filePath); err != nil {
		//  c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		//  return
		// }

		objectKey := fmt.Sprintf("uploads/%s", req.Filename)
		err := bucket.DeleteObject(objectKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
	})

	// 静态文件访问
	r.Static("/uploads", "./uploads")

	r.GET("/chat", func(c *gin.Context) {
		cc.ChatLoop(c)
	})

	r.Run(":8080")
}

func (cc *ChatClient) ChatLoop(c *gin.Context) {
	// 将 HTTP 连接升级为 WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Failed to upgrade connection:", err)
		return
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Failed to read message:", err)
			break
		}

		var request struct {
			Content string   `json:"content"`
			Images  []string `json:"images"`
		}

		err = json.Unmarshal(msg, &request)
		if err != nil {
			log.Println("Invalid request format:", err)
			continue
		}

		// 内容不能为空
		if request.Content == "" {
			log.Println("Content is required")
			continue
		}

		// 调用处理函数
		err = cc.processQuery(conn, request.Content, request.Images)
		if err != nil {
			log.Printf("Error processing request: %v", err)
			continue
		}
	}
}

func (cc *ChatClient) processQuery(ws *websocket.Conn, content string, images []string) error {
	multiContent := []openai.ChatMessagePart{}

	// 图片
	for _, image := range images {
		multiContent = append(multiContent, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL: image,
			},
		})
	}

	// 文本
	multiContent = append(multiContent, openai.ChatMessagePart{
		Type: openai.ChatMessagePartTypeText,
		Text: content,
	})

	// 构造消息
	messages := []openai.ChatCompletionMessage{
		{
			Role:         openai.ChatMessageRoleUser,
			MultiContent: multiContent,
		},
	}

	var finalAnswer strings.Builder

	// 开启流式响应
	stream, err := cc.openaiClient.CreateChatCompletionStream(context.Background(), openai.ChatCompletionRequest{
		Model:    cc.model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return err
	}
	defer stream.Close()

	for {
		resp, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) { // 流结束处理

				cc.messages = append(cc.messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: finalAnswer.String(),
				})

				ws.WriteMessage(websocket.TextMessage, []byte("\n\n"))

				log.Println("Stream finished.")
				break
			}
			log.Printf("Stream receive error: %v", err)
			break
		}

		for _, choice := range resp.Choices {
			content := choice.Delta.Content
			if content != "" {
				ws.WriteMessage(websocket.TextMessage, []byte(content))
				finalAnswer.WriteString(content)
			}
		}
	}
	return nil
}

type OSSClient struct {
	Endpoint        string
	AccessKeyID     string
	AccessKeySecret string
	Client          *oss.Client
}

func NewOSSClient(endpoint, accessKeyID, accessKeySecret string) (*OSSClient, error) {
	client, err := oss.New(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("创建 OSS 客户端失败: %v", err)
	}

	return &OSSClient{
		Endpoint:        endpoint,
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		Client:          client,
	}, nil
}

func (ossClient *OSSClient) CreateBucket(bucketName string) (*oss.Bucket, error) {
	exists, _ := ossClient.Client.IsBucketExist(bucketName)
	if !exists {
		if err := ossClient.Client.CreateBucket(bucketName); err != nil {
			return nil, fmt.Errorf("创建 OSS 桶失败: %v", err)
		}
	}
	return ossClient.Client.Bucket(bucketName)
}
