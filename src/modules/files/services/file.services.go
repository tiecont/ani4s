package files

import (
	"ani4s/src/config"
	"ani4s/src/utils"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

func FileService(filePath string) (io.Reader, int64, string, *utils.ServiceError) {
	objectKey := strings.TrimPrefix(filePath, "/")
	minioClient := config.MinioClient
	bucketName := config.BucketName
	cacheKey := "image_cache:" + objectKey

	// 1. Try to get from Redis cache
	cached, err := config.RDB.Get(config.Ctx, cacheKey).Bytes()
	if err == nil && len(cached) > 0 {
		log.Printf("[CACHE HIT] %s", cacheKey)
		contentType := http.DetectContentType(cached)
		return bytes.NewReader(cached), int64(len(cached)), contentType, nil
	}
	log.Printf("[CACHE MISS] %s", cacheKey)

	// 2. Try to get from MinIO
	obj, err := minioClient.GetObject(context.Background(), bucketName, objectKey, minio.GetObjectOptions{})
	if err == nil {
		stat, err := obj.Stat()
		if err == nil {
			// Read all content to cache it
			data, err := io.ReadAll(obj)
			if err == nil {
				// Set to Redis (with TTL if desired)
				_ = config.RDB.Set(config.Ctx, cacheKey, data, 6*time.Hour).Err()
				return bytes.NewReader(data), stat.Size, stat.ContentType, nil
			}
		}
	}

	// 3. Fallback: Try to download
	newPath, err := utils.DownloadImageIfNotExist("https://phimimg.com/" + objectKey)
	if err != nil {
		return nil, 0, "", &utils.ServiceError{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("object not found and failed to download: %s", objectKey),
		}
	}

	// Retry fetch
	obj, err = minioClient.GetObject(context.Background(), bucketName, newPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, "", &utils.ServiceError{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("downloaded but failed to retrieve object: %s", newPath),
		}
	}
	stat, err := obj.Stat()
	if err != nil {
		return nil, 0, "", &utils.ServiceError{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("downloaded object but stat failed: %s", newPath),
		}
	}
	// Cache after download
	data, err := io.ReadAll(obj)
	if err == nil {
		_ = config.RDB.Set(config.Ctx, cacheKey, data, 24*time.Hour).Err()
	}
	return bytes.NewReader(data), int64(len(data)), stat.ContentType, nil
}
