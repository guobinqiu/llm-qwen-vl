package main

import (
	"context"
	"fmt"
	"log"

	"github.com/sashabaranov/go-openai"
)

func main() {
	apiKey := "sk-3c26bc48e75044dd810a0838f18d75f9"
	baseURL := "https://dashscope.aliyuncs.com/compatible-mode/v1"

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL

	client := openai.NewClientWithConfig(config)

	imageURL := "https://aigc-image.bj.bcebos.com/miaobi/5mao/b%276I2U5rOi5bCP5LiD5a2U6Zuo5aSp6IO95ri4546p5ZCXXzE3MjQyNTgyMzcuOTcxMzEyNQ%3D%3D%27/0.png"
	imageURL = "https://pic.rmb.bdstatic.com/bjh/240111/081dd7e38f6b1ad42bae6509919c3d653634.jpeg" // 4只鸟
	imageURL = "https://pics5.baidu.com/feed/0bd162d9f2d3572c09e6decfee70572962d0c30a.jpeg"        // 一群鸟
	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: "qwen-vl-plus", // 12只鸟，qwen-vl-plus识别为8/9只，qwen-vl-max识别为10只，gpt-4o识别为10只
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "你是一个多模态视觉助手。在每次看到图片时，请先对整张图片做一个全面、细致的描述，覆盖所有明显的细节和元素，如数量，颜色，位置，大小，形状，纹理，材质，风格，内容等等。",
			}, // 对一群鸟问天气没有说出鸟的信息，添加一个system prompt试试看
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
							URL: imageURL,
						},
					},
					{
						Type: openai.ChatMessagePartTypeText,
						Text: "图中有几只鸟?",
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
