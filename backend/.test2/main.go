package main

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

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

func (ossClient *OSSClient) PutObject(bucketName, objectKey, filePath string) error {
	// 获取 OSS 桶
	bucket, err := ossClient.Client.Bucket(bucketName)
	if err != nil {
		return fmt.Errorf("获取 OSS 桶 '%s' 失败: %v", bucketName, err)
	}

	err = bucket.PutObjectFromFile(objectKey, filePath)
	if err != nil {
		return fmt.Errorf("上传文件 %s 到 %s 失败: %w", filePath, objectKey, err)
	}
	return nil
}

func main() {
	endpoint := "oss-cn-hangzhou.aliyuncs.com"
	accessKeyID := ""
	accessKeySecret := ""
	bucketName := "llm-qwen-vl2"

	// 创建 OSSClient 实例
	ossClient, err := NewOSSClient(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		log.Fatal(err)
	}

	// 检查桶是否存在或创建桶
	_, err = ossClient.CreateBucket(bucketName)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("成功创建或确认 OSS 桶: %v\n", bucketName)

	// 文件上传
	filePath := "./1752821115_bird.jpeg"
	fileName := filepath.Base(filePath)
	objectKey := fmt.Sprintf("uploads/%s", fileName)

	// 上传文件
	err = ossClient.PutObject(bucketName, objectKey, filePath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("成功上传文件 %v 到 OSS 桶: %v\n", fileName, bucketName)
}
