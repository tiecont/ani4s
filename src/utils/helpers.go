package utils

import (
	"ani4s/src/config"
	movies "ani4s/src/modules/movies/models"
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"

	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
)

func Paginate(total int64, page, perPage int) (map[string]interface{}, error) {
	// Avoid division by zero
	if perPage <= 0 {
		perPage = 1
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	// Determine next and previous pages
	var nextPage, prevPage *int
	if page < totalPages {
		next := page + 1
		nextPage = &next
	}
	if page > 1 {
		prev := page - 1
		prevPage = &prev
	}

	// Construct and return pagination data
	return map[string]interface{}{
		"current_page":   page,
		"items_per_page": perPage,
		"next_page":      nextPage,
		"previous_page":  prevPage,
		"total_count":    total,
		"total_pages":    totalPages,
	}, nil
}

// ConvertStringToInt64 is the function will receive a string and return int64
func ConvertStringToInt64(str string) int64 {
	i, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		panic(err)
	}
	return i
}

// ConvertInt64ToString is the function will receive an int64 and return string
func ConvertInt64ToString(i int64) string {
	return strconv.FormatInt(i, 10)
}

// ConvertInt64ToUint is the function will receive an int64 and return uint format
func ConvertInt64ToUint(i int64) uint {
	return uint(i)
}

// ConvertStringToUint is the function to convert string to uint format
func ConvertStringToUint(str string) uint {
	i, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		panic(err)
	}
	return uint(i)
}

// ServiceError to define return exception for system
type ServiceError struct {
	StatusCode int
	Message    string
}

func (e *ServiceError) Error() string {
	return e.Message
}

// CalculateOffsetStruct is the struct to define return result for calculate service
type CalculateOffsetStruct struct {
	CurrentPage  int
	ItemsPerPage int
	OrderBy      string
	SortBy       string
	Offset       int
}

// CalculateOffset is the function to calculate offset for list service
func CalculateOffset(currentPage, itemsPerPage int, sortBy, orderBy string) CalculateOffsetStruct {
	if orderBy == "" {
		orderBy = "created_at"
	}
	if sortBy != "asc" && sortBy != "desc" {
		sortBy = "desc"
	}

	// Calculate offset for pagination
	offset := (currentPage - 1) * itemsPerPage
	if offset < 0 {
		offset = 0
	}

	return CalculateOffsetStruct{
		CurrentPage:  currentPage,
		ItemsPerPage: itemsPerPage,
		OrderBy:      orderBy,
		SortBy:       sortBy,
		Offset:       offset,
	}
}

// BindJson is a function to bind the json request
func BindJson(c *gin.Context, request interface{}) *ServiceError {
	if err := c.ShouldBind(&request); err != nil {
		return &ServiceError{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid input",
		}
	}
	return nil
}

func IsValidApiResponse(raw map[string]interface{}) (map[string]interface{}, bool) {
	// Nếu đã có data và data chứa item
	if data, ok := raw["data"]; ok {
		if dmap, ok := data.(map[string]interface{}); ok {
			if items, hasItem := dmap["items"]; hasItem {
				enriched := EnrichThumbFromDatabase(items)
				dmap["items"] = enriched
				return raw, true
			}
		}
	}

	// Nếu "items" ở root
	if items, ok := raw["items"]; ok {
		pagination := raw["pagination"]

		enriched := EnrichThumbFromDatabase(items)

		data := map[string]interface{}{
			"items": enriched,
		}
		if pagination != nil {
			data["pagination"] = pagination
		}
		return map[string]interface{}{
			"data": data,
		}, true
	}

	// Danh sách key cần kiểm tra
	candidates := []string{"episodes", "movies", "list", "data"}

	for _, key := range candidates {
		if val, exists := raw[key]; exists {
			enriched := EnrichThumbFromDatabase(val)
			return map[string]interface{}{
				"data": map[string]interface{}{
					"items": map[string]interface{}{
						key: enriched,
					},
				},
			}, true
		}
	}

	return nil, false
}

func EnrichThumbFromDatabase(items interface{}) interface{} {
	db := config.DB
	list, ok := items.([]interface{})
	if !ok {
		return items
	}

	// Bước 1: Collect all valid IDs
	var ids []string
	for _, i := range list {
		if m, ok := i.(map[string]interface{}); ok {
			if id, ok := m["_id"].(string); ok && id != "" {
				ids = append(ids, id)
			}
		}
	}

	if len(ids) == 0 {
		return items
	}

	// Bước 2: Query từ DB (Movie.ID -> ThumbURL)
	type Result struct {
		ID       string
		ThumbURL string
	}
	var dbResults []Result
	if err := db.
		Model(&movies.Movie{}).
		Select("id", "thumb_url").
		Where("id IN ?", ids).
		Find(&dbResults).Error; err != nil {
		fmt.Printf("[EnrichThumb] DB error: %v\n", err)
		return items
	}

	// Bước 3: Dùng map tra nhanh
	thumbMap := make(map[string]string, len(dbResults))
	for _, r := range dbResults {
		if r.ThumbURL != "" {
			thumbMap[r.ID] = r.ThumbURL
		}
	}

	// Bước 4: Duyệt danh sách gốc và cập nhật nếu có DB match
	for _, i := range list {
		if m, ok := i.(map[string]interface{}); ok {
			if id, ok := m["_id"].(string); ok {
				if newThumb, exists := thumbMap[id]; exists {
					m["thumb_url"] = newThumb
				}
			}
		}
	}

	return list
}

func TranslateToEnglish(text string) (string, error) {
	payload := map[string]string{
		"q":      text,
		"source": "vi",
		"target": "en",
		"format": "text",
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post("https://libretranslate.com/translate", "application/json", bytes.NewReader(body))
	if err != nil {
		return text, err
	}
	defer resp.Body.Close()

	data, _ := ioutil.ReadAll(resp.Body)
	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		return text, err
	}
	return result["translatedText"], nil
}

func TranslateValuesInItems(items []interface{}) {
	for _, item := range items {
		if video, ok := item.(map[string]interface{}); ok {
			if name, ok := video["name"].(string); ok {
				if enName, err := TranslateToEnglish(name); err == nil {
					video["name_en"] = enName
				}
			}
			if origin, ok := video["origin_name"].(string); ok {
				if enOrigin, err := TranslateToEnglish(origin); err == nil {
					video["origin_name_en"] = enOrigin
				}
			}
		}
	}
}

// DownloadImageIfNotExist downloads an image if it's phimimg.com and not in local
func DownloadImageIfNotExist(url string) (string, error) {
	const prefix = "https://phimimg.com/"
	if !strings.HasPrefix(url, prefix) {
		return url, nil
	}

	minioClient := config.MinioClient
	bucketName := config.BucketName

	relativePath := strings.TrimPrefix(url, prefix)

	// Kiểm tra object đã tồn tại chưa
	_, err := minioClient.StatObject(context.Background(), bucketName, relativePath, minio.GetObjectOptions{})
	if err == nil {
		return relativePath, nil // đã có ảnh
	}

	// Tải ảnh từ URL
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status when downloading image: %d", resp.StatusCode)
	}

	// Kiểm tra và tạo bucket nếu chưa có
	exists, err := minioClient.BucketExists(context.Background(), bucketName)
	if err != nil {
		return "", fmt.Errorf("failed to check/create bucket: %w", err)
	}
	if !exists {
		err = minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	// Lưu ảnh lên MinIO bucket
	_, err = minioClient.PutObject(
		context.Background(),
		bucketName,
		relativePath,
		resp.Body,
		resp.ContentLength,
		minio.PutObjectOptions{ContentType: resp.Header.Get("Content-Type")},
	)
	if err != nil {
		return "", fmt.Errorf("failed to upload image to minio: %w", err)
	}

	return relativePath, nil
}
