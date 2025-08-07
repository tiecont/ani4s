package movies

import (
	"ani4s/src/config"
	movies "ani4s/src/modules/movies/lib"
	"ani4s/src/utils"
	"encoding/json"
	"fmt"
	"golang.org/x/sync/singleflight"
	"net/url"
	"time"
)

var requestGroup singleflight.Group

func GetMovieList(req movies.MovieListRequest) (map[string]interface{}, error) {
	rdb := config.RDB
	ctx := config.Ctx

	cacheKey := buildCacheKey(req)

	// 1. Try Redis cache
	if cached, err := rdb.Get(ctx, cacheKey).Bytes(); err == nil && len(cached) > 0 {
		var result map[string]interface{}
		if jsonErr := json.Unmarshal(cached, &result); jsonErr == nil {
			result["from_cache"] = true
			return result, nil
		}
	}

	// 2. Use singleflight to prevent duplicate API calls
	rawResult, err, _ := requestGroup.Do(cacheKey, func() (interface{}, error) {
		// 2a. Fetch from remote API
		targetURL := buildListURL(req)
		body, err := MakeAnonymousRequest(targetURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch from upstream API: %w", err)
		}

		var apiResponse map[string]interface{}
		if err := json.Unmarshal(body, &apiResponse); err != nil {
			return nil, fmt.Errorf("invalid JSON from upstream: %w", err)
		}

		normalized, ok := utils.IsValidApiResponse(apiResponse)
		if !ok {
			return map[string]interface{}{
				"success": false,
				"error":   "Unexpected response structure, skip caching",
			}, nil
		}

		// Add metadata
		normalized["from_cache"] = false
		normalized["request_url"] = targetURL
		normalized["timestamp"] = time.Now().Unix()

		// 2c. Encode to JSON once
		jsonBytes, _ := json.Marshal(apiResponse)

		// 2d. Save to Redis with pipeline
		ttl := 16 * time.Hour
		tagKey := "movie_list:cached_keys"

		pipe := rdb.Pipeline()
		pipe.Set(ctx, cacheKey, jsonBytes, ttl)
		pipe.SAdd(ctx, tagKey, cacheKey)
		pipe.Expire(ctx, tagKey, 12*time.Hour)
		_, _ = pipe.Exec(ctx)

		return apiResponse, nil
	})

	if err != nil {
		return nil, err
	}

	return rawResult.(map[string]interface{}), nil
}

// GetSearchMovies performs a search query using phimapi.com and caches the response
func GetSearchMovies(req movies.MovieSearchRequest) (map[string]interface{}, error) {
	rdb := config.RDB
	ctx := config.Ctx

	targetURL := buildSearchURL(req)
	cacheKey := buildSearchCacheKey(req)

	// 1. Redis cache first
	if cached, err := rdb.Get(ctx, cacheKey).Bytes(); err == nil && len(cached) > 0 {
		var result map[string]interface{}
		if jsonErr := json.Unmarshal(cached, &result); jsonErr == nil {
			result["from_cache"] = true
			return result, nil
		}
	}

	// 2. Fetch from upstream
	responseBody, err := MakeAnonymousRequest(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch search results from API: %w", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(responseBody, &raw); err != nil {
		return map[string]interface{}{
			"success":  false,
			"error":    "Failed to parse API response",
			"raw_data": string(responseBody),
		}, nil
	}

	// 3. Normalize response
	normalized, ok := utils.IsValidApiResponse(raw)
	if !ok {
		return map[string]interface{}{
			"success": false,
			"error":   "Unexpected response structure, skip caching",
		}, nil
	}

	// 4. Metadata
	normalized["from_cache"] = false
	normalized["request_url"] = targetURL
	normalized["timestamp"] = time.Now().Unix()

	// 5. Marshal once
	jsonStr, _ := json.Marshal(normalized)

	// 6. Cache with shorter TTL for search
	ttl := 8 * time.Hour
	tagKey := "movie_search:cached_keys"

	pipe := rdb.Pipeline()
	pipe.Set(ctx, cacheKey, jsonStr, ttl)
	pipe.SAdd(ctx, tagKey, cacheKey)
	pipe.Expire(ctx, tagKey, 12*time.Hour)
	_, _ = pipe.Exec(ctx)

	// 7. Return final map
	var returnMap map[string]interface{}
	_ = json.Unmarshal(jsonStr, &returnMap)
	return returnMap, nil
}

// buildListURL constructs the target API URL with query params
func buildListURL(req movies.MovieListRequest) string {
	baseURL := fmt.Sprintf("https://phimapi.com/v1/api/danh-sach/%s", req.TypeList)
	params := url.Values{}
	params.Add("page", fmt.Sprintf("%d", req.Page))
	params.Add("sort_field", req.SortField)
	params.Add("sort_type", req.SortType)
	params.Add("limit", fmt.Sprintf("%d", req.Limit))
	if req.SortLang != "" {
		params.Add("sort_lang", req.SortLang)
	}
	if req.Category != "" {
		params.Add("category", req.Category)
	}
	if req.Country != "" {
		params.Add("country", req.Country)
	}
	if req.Year != 0 {
		params.Add("year", fmt.Sprintf("%d", req.Year))
	}
	return baseURL + "?" + params.Encode()
}

// buildCacheKey creates a unique Redis cache key for a list query
func buildCacheKey(req movies.MovieListRequest) string {
	return fmt.Sprintf("movie_list:%s:%d:%s:%s:%s:%s:%s:%d:%d",
		req.TypeList, req.Page, req.SortField, req.SortType,
		req.SortLang, req.Category, req.Country, req.Year, req.Limit)
}

func buildSearchURL(req movies.MovieSearchRequest) string {
	baseURL := "https://phimapi.com/v1/api/tim-kiem"
	params := url.Values{}
	params.Add("keyword", req.Keyword)
	params.Add("page", fmt.Sprintf("%d", req.Page))
	params.Add("sort_field", req.SortField)
	params.Add("sort_type", req.SortType)
	params.Add("limit", fmt.Sprintf("%d", req.Limit))

	if req.SortLang != "" {
		params.Add("sort_lang", req.SortLang)
	}
	if req.Category != "" {
		params.Add("category", req.Category)
	}
	if req.Country != "" {
		params.Add("country", req.Country)
	}
	if req.Year != 0 {
		params.Add("year", fmt.Sprintf("%d", req.Year))
	}

	return baseURL + "?" + params.Encode()
}

func buildSearchCacheKey(req movies.MovieSearchRequest) string {
	return fmt.Sprintf("movie_search:%s:%d:%s:%s:%s:%s:%s:%d:%d",
		req.Keyword, req.Page, req.SortField, req.SortType,
		req.SortLang, req.Category, req.Country, req.Year, req.Limit)
}
