package services

import (
	"ani4s/src/config"
	movies "ani4s/src/modules/movies/services"
	"encoding/json"
	"fmt"
	"github.com/robfig/cron/v3"
	"log"
)

// SetupBackgroundJobs sets up and starts background jobs like syncing movie details from cached keys.
func SetupBackgroundJobs() {
	c := cron.New()

	tagKeys := []string{
		"movie_list:cached_keys",
		"movie_search:cached_keys",
		"trending:cached_keys",
		"movie_category:cached_keys",
	}

	for _, tagKey := range tagKeys {
		tk := tagKey // tránh capture biến loop
		c.AddFunc("@every 15m", func() {
			syncMoviesFromCacheByTagKey(tk)
		})
	}

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
				fmt.Print(slug)
				if slug != "" {
					log.Printf("[Sync] Syncing details for slug: %s", slug)
					_, err := movies.GetDetailsMovie(slug)
					if err != nil {
						log.Printf("[Sync] Error syncing slug %s: %v", slug, err)
					}
				}
			}
		}
	}

	log.Printf("[Sync] Finished sync for tagKey: %s", tagKey)
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
