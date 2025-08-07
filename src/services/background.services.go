package services

import (
	"ani4s/src/config"
	file "ani4s/src/modules/files/services"
	movies2 "ani4s/src/modules/movies/models"
	movies "ani4s/src/modules/movies/services"
	"ani4s/src/utils"
	"net/url"

	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/robfig/cron/v3"
)

// SetupBackgroundJobs sets up and starts background jobs like syncing movie details from cached keys.
func SetupBackgroundJobs() {
	c := cron.New()

	tagKeys := []string{
		"movie_list:cached_keys",
		"movie_search:cached_keys",
		"trending:cached_keys",
		"movie_category:cached_keys",
		"movie_country:cached_keys",
		"movie_newest:cached_keys",
	}

	for _, tagKey := range tagKeys {
		tk := tagKey // tránh capture biến loop
		c.AddFunc("@every 15m", func() {
			go syncMoviesFromCacheByTagKey(tk)
		})
	}
	c.AddFunc("@every 10m", func() {
		go FetchAndUpdateThumbnails()
	})

	c.Start()
	log.Println("[Cron] Background jobs initialized with tag keys:", tagKeys)
}

// syncMoviesFromCacheByTagKey reads cached movie list keys from a specific tagKey and syncs details.
func syncMoviesFromCacheByTagKey(tagKey string) {
	rdb := config.RDB
	ctx := config.Ctx
	log.Printf("[Sync] Starting background movie detail sync from Redis tag key: %s", tagKey)

	// Step 1: Get all cache keys in the given tag set
	keys, err := rdb.SMembers(ctx, tagKey).Result()
	if err != nil || len(keys) == 0 {
		log.Printf("[Sync] No keys found or error occurred while reading Redis tag set %s: %v", tagKey, err)
		return
	}
	for _, cacheKey := range keys {
		cached, err := rdb.Get(ctx, cacheKey).Result()
		if err != nil {
			log.Printf("[Sync] Failed to read cache key %s: %v", cacheKey, err)
			continue
		}

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(cached), &data); err != nil {
			log.Printf("[Sync] Failed to unmarshal cached data for key %s: %v", cacheKey, err)
			continue
		}

		// Try to detect structure: ["data"]["items"] for list/search results
		items, ok := extractItemsFromData(data)
		if !ok {
			log.Printf("[Sync] Could not find 'data.items' in cache key %s", cacheKey)
			continue
		}

		for _, item := range items {
			if movieMap, ok := item.(map[string]interface{}); ok {
				slug, _ := movieMap["slug"].(string)
				if slug != "" {
					log.Printf("[Sync] Syncing details for slug: %s", slug)
					res, err := movies.GetDetailsMovie(slug)
					if err != nil {
						log.Printf("[Sync] Error syncing slug %s: %v", slug, err)
						continue
					}

					// Truy cập res["data"]["movie"]["thumb_url"]
					dataMap, ok := res["data"].(map[string]interface{})
					if !ok {
						log.Printf("[Sync] Invalid data format for slug %s", slug)
						continue
					}

					movieData, ok := dataMap["movie"].(map[string]interface{})
					if !ok {
						log.Printf("[Sync] Invalid movie format for slug %s", slug)
						continue
					}

					thumbURL, _ := movieData["thumb_url"].(string)
					if thumbURL == "" {
						log.Printf("[Sync] Empty thumb_url for slug %s", slug)
						continue
					}

					err = syncImage(thumbURL)
					if err != nil {
						log.Printf("[Sync] Error syncing image from thumbnail: %v", err)
					}
				}
			}
		}
	}

	log.Printf("[Sync] Finished sync for tagKey: %s", tagKey)
}

func syncImage(thumb string) error {
	if thumb == "" || thumb == "/" {
		return fmt.Errorf("invalid thumbnail URL")
	}

	// Nếu thumb là URL đầy đủ (http/https), trích xuất phần path
	cleanThumb := thumb
	if strings.HasPrefix(thumb, "http://") || strings.HasPrefix(thumb, "https://") {
		parsedURL, err := url.Parse(thumb)
		if err != nil {
			return fmt.Errorf("failed to parse thumbnail URL: %v", err)
		}
		cleanThumb = parsedURL.Path
	}

	// Loại bỏ dấu `/` thừa ở đầu
	cleanThumb = strings.TrimPrefix(cleanThumb, "/")

	// Gọi FileService với path đã chuẩn hoá
	_, _, _, err := file.FileService(cleanThumb)
	if err != nil {
		return fmt.Errorf("syncImage error: %v", err)
	}

	return nil
}

func extractItemsFromData(data map[string]interface{}) ([]interface{}, bool) {
	rawData, ok := data["data"]
	if !ok {
		return nil, false
	}

	// Nếu rawData là slice luôn (trường hợp search)
	if items, ok := rawData.([]interface{}); ok {
		return items, true
	}

	// Nếu là object có "items" (trường hợp danh sách)
	if m, ok := rawData.(map[string]interface{}); ok {
		if items, ok := m["items"].([]interface{}); ok {
			return items, true
		}
	}

	return nil, false
}

func FetchAndUpdateThumbnails() {
	var movies []movies2.Movie
	if err := config.DB.Select("id", "slug", "thumb_url", "poster_url").Find(&movies).Error; err != nil {
		fmt.Printf("[ImageSync] Error fetching movies: %v\n", err)
		return
	}

	fmt.Printf("[ImageSync] Found %d movies to process\n", len(movies))

	for _, movie := range movies {
		processImageField(movie.Slug, "thumb_url", movie.ThumbURL)
		processImageField(movie.Slug, "poster_url", movie.PosterURL)
	}
}

func processImageField(slug string, fieldName string, originalURL string) {
	if originalURL == "" || originalURL == "/" {
		fmt.Printf("[ImageSync] Skipping empty %s for slug %s\n", fieldName, slug)
		return
	}

	var remoteURL string

	switch {
	case strings.HasPrefix(originalURL, "http"):
		// Dạng full URL → trích xuất phần path
		parsed, err := url.Parse(originalURL)
		if err != nil {
			fmt.Printf("[ImageSync] Invalid URL in %s for slug %s: %v\n", fieldName, slug, err)
			return
		}
		remoteURL = fmt.Sprintf("https://phimimg.com%s", parsed.Path)

	case strings.HasPrefix(originalURL, "upload/"), strings.HasPrefix(originalURL, "/upload/"):
		cleanPath := strings.TrimPrefix(originalURL, "/")
		remoteURL = fmt.Sprintf("https://phimimg.com/%s", cleanPath)

	default:
		fmt.Printf("[ImageSync] Unrecognized %s format for slug %s: %s\n", fieldName, slug, originalURL)
		return
	}

	newPath, err := utils.DownloadImageIfNotExist(remoteURL)
	if err != nil {
		fmt.Printf("[ImageSync] Error downloading %s for slug %s: %v\n", fieldName, slug, err)
		return
	}

	// Nếu URL đã thay đổi → cập nhật DB
	if newPath != originalURL {
		if err := updateImageField(slug, fieldName, newPath); err != nil {
			fmt.Printf("[ImageSync] Failed to update %s for slug %s: %v\n", fieldName, slug, err)
			return
		}
		fmt.Printf("[ImageSync] Updated %s for slug %s to %s\n", fieldName, slug, newPath)
	} else {
		fmt.Printf("[ImageSync] %s for slug %s already up to date\n", fieldName, slug)
	}

	if err := syncImage(newPath); err != nil {
		fmt.Printf("[ImageSync] syncImage failed for slug %s: %v\n", slug, err)
	}
}

func updateImageField(slug, field, newPath string) error {
	return config.DB.Model(&movies2.Movie{}).Where("slug = ?", slug).Update(field, newPath).Error
}
