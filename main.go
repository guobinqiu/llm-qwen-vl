package main

import (
	"context"
	"fmt"
	"log"

	"github.com/sashabaranov/go-openai"
)

func main() {
	apiKey := ""
	baseURL := "https://dashscope.aliyuncs.com/compatible-mode/v1"

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL

	client := openai.NewClientWithConfig(config)

	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: "qwen-vl-plus",
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: "图中什么天气?",
					},
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL: "https://aigc-image.bj.bcebos.com/miaobi/5mao/b%276I2U5rOi5bCP5LiD5a2U6Zuo5aSp6IO95ri4546p5ZCXXzE3MjQyNTgyMzcuOTcxMzEyNQ%3D%3D%27/0.png",
						},
					},
				},
			},
		},
	})

	if err != nil {
		log.Fatalf("请求出错: %v", err)
	}

	fmt.Println("回答:", resp.Choices[0].Message.Content)
}
