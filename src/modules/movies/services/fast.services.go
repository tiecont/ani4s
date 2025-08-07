package movies

import (
	"ani4s/src/config"
	movies "ani4s/src/modules/movies/models"
	"encoding/json"
	"fmt"
	"time"
)

func ListAllCategories() (map[string]interface{}, error) {
	db := config.DB
	rdb := config.RDB
	ctx := config.Ctx

	targetURL := "https://phimapi.com/the-loai"
	cacheKey := fmt.Sprintf("categories:%s", targetURL)

	// 1. Redis Cache
	if cached, err := rdb.Get(ctx, cacheKey).Bytes(); err == nil && len(cached) > 0 {
		var result map[string]interface{}
		if jsonErr := json.Unmarshal(cached, &result); jsonErr == nil {
			if rawData, ok := result["data"].([]interface{}); ok {
				for _, item := range rawData {
					if cat, ok := item.(map[string]interface{}); ok {
						if cat["slug"] == "mien-tay" {
							cat["name"] = "Cao Bồi"
						}
					}
				}
			}
			result["from_cache"] = true
			return result, nil
		}
	}

	// 2. Fetch from API
	responseBody, err := MakeAnonymousRequest(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch categories: %w", err)
	}

	// 3. Parse JSON array
	var rawCategories []movies.Category
	if err := json.Unmarshal(responseBody, &rawCategories); err != nil {
		return map[string]interface{}{
			"success":  false,
			"error":    "Failed to parse API response",
			"raw_data": string(responseBody),
		}, nil
	}

	// ✅ 3.5. Override "Miền Tây" → "Cao Bồi"
	for i := range rawCategories {
		if rawCategories[i].Slug == "mien-tay" {
			rawCategories[i].Name = "Cao Bồi"
		}
	}

	// 4. Sync categories to DB
	for _, cat := range rawCategories {
		_ = db.FirstOrCreate(&movies.Category{}, movies.Category{ID: cat.ID}).Error
	}

	// 5. Prepare normalized response
	result := map[string]interface{}{
		"success":     true,
		"data":        rawCategories,
		"from_cache":  false,
		"request_url": targetURL,
		"timestamp":   time.Now().Unix(),
	}

	// 6. Cache response
	if jsonBytes, err := json.Marshal(result); err == nil {
		_ = rdb.Set(ctx, cacheKey, jsonBytes, 23*time.Hour).Err()
	}

	return result, nil
}

func ListAllCountry() (map[string]interface{}, error) {
	db := config.DB
	rdb := config.RDB
	ctx := config.Ctx

	targetURL := "https://phimapi.com/quoc-gia"
	cacheKey := fmt.Sprintf("countries:%s", targetURL)

	// 1. Redis Cache
	if cached, err := rdb.Get(ctx, cacheKey).Bytes(); err == nil && len(cached) > 0 {
		var result map[string]interface{}
		if jsonErr := json.Unmarshal(cached, &result); jsonErr == nil {
			result["from_cache"] = true
			return result, nil
		}
	}

	// 2. Fetch from API
	responseBody, err := MakeAnonymousRequest(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch countries: %w", err)
	}

	// 3. Parse JSON array
	var rawCountry []movies.Country
	if err := json.Unmarshal(responseBody, &rawCountry); err != nil {
		return map[string]interface{}{
			"success":  false,
			"error":    "Failed to parse API response",
			"raw_data": string(responseBody),
		}, nil
	}

	// 4. Sync categories to DB
	for _, cat := range rawCountry {
		_ = db.FirstOrCreate(&movies.Country{}, movies.Country{Name: cat.Name}).Error
	}

	// 5. Prepare normalized response
	result := map[string]interface{}{
		"success":     true,
		"data":        rawCountry,
		"from_cache":  false,
		"request_url": targetURL,
		"timestamp":   time.Now().Unix(),
	}

	// 6. Cache response
	if jsonBytes, err := json.Marshal(result); err == nil {
		_ = rdb.Set(ctx, cacheKey, jsonBytes, 23*time.Hour).Err()
	}

	return result, nil
}
