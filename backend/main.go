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

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sashabaranov/go-openai"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 允许所有来源
		return true
	},
}

func main() {
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

	r.POST("/upload", func(c *gin.Context) {
		file, err := c.FormFile("image")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		filename := fmt.Sprintf("uploads/%d_%s", time.Now().Unix(), filepath.Base(file.Filename))
		err = c.SaveUploadedFile(file, filename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 假设静态地址
		c.JSON(http.StatusOK, gin.H{
			"errno": 0,
			"data": []string{
				"http://localhost:8080/" + filename,
			},
		})
	})

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

		filePath := filepath.Join("uploads", req.Filename)
		if err := os.Remove(filePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
	})

	// 静态文件访问
	r.Static("/uploads", "./uploads")

	r.GET("/chat", func(c *gin.Context) {
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
			err = processQuery(conn, request.Content, request.Images)
			if err != nil {
				log.Printf("Error processing request: %v", err)
				continue
			}
		}
	})

	r.Run(":8080")
}
func processQuery(ws *websocket.Conn, content string, images []string) error {
	apiKey := "sk-3c26bc48e75044dd810a0838f18d75f9"
	baseURL := "https://dashscope.aliyuncs.com/compatible-mode/v1"

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL

	client := openai.NewClientWithConfig(config)

	multiContent := []openai.ChatMessagePart{}

	// 图片
	if len(images) > 0 {
		for _, image := range images {
			multiContent = append(multiContent, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL: image,
				},
			})
		}
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

	// 开启流式响应
	stream, err := client.CreateChatCompletionStream(context.Background(), openai.ChatCompletionRequest{
		Model:    "qwen-vl-plus",
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
			}
		}
	}
	return nil
}
