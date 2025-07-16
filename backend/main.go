package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

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

	// 多轮对话接口（调用 Qwen）
	r.POST("/chat", func(c *gin.Context) {
		type Request struct {
			Content string   `json:"content"`
			Images  []string `json:"images"`
		}
		var req Request
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		reply, err := callQwenAPI(req.Content, req.Images)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"reply": reply})
	})

	r.Run(":8080")
}

func callQwenAPI(content string, images []string) (string, error) {
	apiKey := ""
	baseURL := "https://dashscope.aliyuncs.com/compatible-mode/v1"

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL

	client := openai.NewClientWithConfig(config)

	multiContent := []openai.ChatMessagePart{}
	for _, image := range images {
		multiContent = append(multiContent, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL: image,
			},
		})
	}

	multiContent = append(multiContent, openai.ChatMessagePart{
		Type: openai.ChatMessagePartTypeText,
		Text: content,
	})

	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: "qwen-vl-plus",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "你是一个多模态视觉助手。在每次看到图片时，请先对整张图片做一个全面、细致的描述，覆盖所有明显的细节和元素，如数量，颜色，位置，大小，形状，纹理，材质，风格，内容等等。",
			},
			{
				Role:         openai.ChatMessageRoleUser,
				MultiContent: multiContent,
			},
		},
	})

	if err != nil {
		fmt.Println(err)
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}
