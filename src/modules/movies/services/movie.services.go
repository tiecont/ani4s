package movies

import (
	"ani4s/src/config"
	lib "ani4s/src/modules/movies/lib"
	movies "ani4s/src/modules/movies/models"
	"ani4s/src/utils"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"gorm.io/gorm/clause"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MakeAnonymousRequest makes a GET request while masking server information
func MakeAnonymousRequest(url string) ([]byte, error) {
	// Create a custom HTTP client with modifications to hide server info
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
			DisableKeepAlives: true, // Prevent connection reuse
		},
	}

	// Create the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add headers to mask the request origin and mimic a regular browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	// Remove or mask headers that might reveal server information
	req.Header.Del("X-Forwarded-For")
	req.Header.Del("X-Real-IP")
	req.Header.Del("X-Forwarded-Proto")
	req.Header.Del("X-Forwarded-Host")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle compressed responses
	var reader io.Reader = resp.Body

	// Check if response is gzip compressed
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Read the response body
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func GetListNewestMovies(page, v int) (map[string]interface{}, error) {
	rdb := config.RDB
	ctx := config.Ctx

	// Xây dựng URL
	var targetURL string
	if v == 1 {
		targetURL = fmt.Sprintf("https://phimapi.com/danh-sach/phim-moi-cap-nhat?page=%d", page)
	} else {
		targetURL = fmt.Sprintf("https://phimapi.com/danh-sach/phim-moi-cap-nhat-v%d?page=%d", v, page)
	}

	cacheKey := fmt.Sprintf("movie_newest:%s", targetURL)

	// 1. Thử lấy từ Redis cache
	if cached, err := rdb.Get(ctx, cacheKey).Bytes(); err == nil && len(cached) > 0 {
		var result map[string]interface{}
		if jsonErr := json.Unmarshal(cached, &result); jsonErr == nil {
			result["from_cache"] = true
			return result, nil
		}
	}

	// 2. Gọi API gốc
	responseBody, err := MakeAnonymousRequest(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch newest movie list: %w", err)
	}

	// 3. Parse JSON dạng bất kỳ
	var raw map[string]interface{}
	if err := json.Unmarshal(responseBody, &raw); err != nil {
		return map[string]interface{}{
			"success":  false,
			"error":    "Failed to parse API response",
			"raw_data": string(responseBody),
		}, nil
	}

	// 4. Normalize theo chuẩn `data.item`
	normalized, ok := utils.IsValidApiResponse(raw)
	if !ok {
		return map[string]interface{}{
			"success": false,
			"error":   "Unexpected response structure",
		}, nil
	}

	// 5. Metadata + cache
	normalized["from_cache"] = false
	normalized["request_url"] = targetURL
	normalized["timestamp"] = time.Now().Unix()

	jsonStr, _ := json.Marshal(normalized)
	ttl := 16 * time.Hour
	tagKey := "movie_newest:cached_keys"

	pipe := rdb.Pipeline()
	pipe.Set(ctx, cacheKey, jsonStr, ttl)
	pipe.SAdd(ctx, tagKey, cacheKey)
	pipe.Expire(ctx, tagKey, ttl)
	_, _ = pipe.Exec(ctx)

	// 6. Trả về map
	var returnMap map[string]interface{}
	_ = json.Unmarshal(jsonStr, &returnMap)

	return returnMap, nil
}

func GetDetailsMovie(slug string) (map[string]interface{}, error) {
	rdb := config.RDB
	ctx := config.Ctx
	db := config.DB

	targetURL := fmt.Sprintf("https://phimapi.com/phim/%s", slug)
	cacheKey := fmt.Sprintf("movie_details:%s", targetURL)

	// 1. Check Redis cache
	if cached, err := rdb.Get(ctx, cacheKey).Bytes(); err == nil {
		var result map[string]interface{}
		if jsonErr := json.Unmarshal(cached, &result); jsonErr == nil {
			result["from_cache"] = true
			return result, nil
		}
	}

	// 2. Check local DB
	if res, err := GetMovieDetailsFromDB(slug); err == nil {
		res["from_cache"] = false
		return res, nil
	} else {
		fmt.Printf("[MovieDetails] Fallback to API for slug %s due to DB error: %v\n", slug, err)
	}

	// 3. Fetch from external API
	responseBody, err := MakeAnonymousRequest(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch movie details: %w", err)
	}

	// 4. Parse API response
	var data movies.MovieDetails
	if err := json.Unmarshal(responseBody, &data); err != nil {
		return map[string]interface{}{
			"success":  false,
			"error":    "Invalid API response",
			"raw_data": string(responseBody),
		}, nil
	}

	// 5. Save to DB with transaction
	tx := db.Begin()

	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(&data.Movie).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to upsert movie: %w", err)
	}

	for _, group := range data.Episodes {
		for _, ep := range group.ServerData {
			ep.MovieID = data.Movie.ID
			ep.Movie = movies.Movie{}
			ep.ServerName = group.ServerName

			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "slug"}},
				DoNothing: true,
			}).Create(&ep).Error

			if err != nil && !strings.Contains(err.Error(), "duplicate key") {
				tx.Rollback()
				return nil, fmt.Errorf("failed to upsert episode: %w", err)
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("transaction commit failed: %w", err)
	}

	// 6. Build response
	result := map[string]interface{}{
		"data": map[string]interface{}{
			"movie":    data.Movie,
			"episodes": data.Episodes,
		},
		"from_cache":  false,
		"request_url": targetURL,
		"timestamp":   time.Now().Unix(),
	}

	// 7. Save to Redis cache
	if jsonStr, err := json.Marshal(result); err == nil {
		ttl := 16 * time.Hour
		pipe := rdb.Pipeline()
		pipe.Set(ctx, cacheKey, jsonStr, ttl)
		_, _ = pipe.Exec(ctx)
	}

	return result, nil
}

func GetMovieDetailsFromDB(slug string) (map[string]interface{}, error) {
	db := config.DB
	rdb := config.RDB
	ctx := config.Ctx

	targetURL := fmt.Sprintf("https://phimapi.com/phim/%s", slug)
	cacheKey := fmt.Sprintf("movie_details:%s", targetURL)

	// 1. Movie
	var movie movies.Movie
	if err := db.Preload("Categories").Preload("Countries").Where("slug = ?", slug).First(&movie).Error; err != nil {
		return nil, err
	}

	// 2. Episodes
	var allEpisodes []movies.Episode
	if err := db.Where("movie_id = ?", movie.ID).Find(&allEpisodes).Error; err != nil {
		return nil, fmt.Errorf("failed to load episodes: %w", err)
	}

	// ❗️Nếu không có episodes => xoá Redis và trả lỗi để fallback sang API
	if len(allEpisodes) == 0 {
		_ = rdb.Del(ctx, cacheKey).Err()
		return nil, fmt.Errorf("no episodes found for movie slug: %s", slug)
	}

	// 3. Group episodes
	grouped := make(map[string][]movies.Episode)
	for _, ep := range allEpisodes {
		ep.Movie = movies.Movie{} // tránh circular reference
		grouped[ep.ServerName] = append(grouped[ep.ServerName], ep)
	}

	var episodeGroups []movies.EpisodeGroup
	for server, eps := range grouped {
		episodeGroups = append(episodeGroups, movies.EpisodeGroup{
			ServerName: server,
			ServerData: eps,
		})
	}

	// 4. Format response
	result := map[string]interface{}{
		"data": map[string]interface{}{
			"movie":    movie,
			"episodes": episodeGroups,
		},
		"from_cache":  true,
		"request_url": targetURL,
		"timestamp":   time.Now().Unix(),
	}

	// 5. Lưu vào Redis
	if jsonStr, err := json.Marshal(result); err == nil {
		ttl := 16 * time.Hour
		tagKey := "movie_details:cached_keys"

		pipe := rdb.Pipeline()
		pipe.Set(ctx, cacheKey, jsonStr, ttl)
		pipe.SAdd(ctx, tagKey, cacheKey)
		pipe.Expire(ctx, tagKey, ttl)
		_, _ = pipe.Exec(ctx)
	}

	return result, nil
}

func ListMoviesByCategory(req lib.MoviesByCategoryRequest) (map[string]interface{}, error) {
	rdb := config.RDB
	ctx := config.Ctx

	targetURL := buildCategoryURL(req)
	cacheKey := buildCategoryCacheKey(req)

	// 1. Redis cache
	if cached, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		var cachedData map[string]interface{}
		if err := json.Unmarshal([]byte(cached), &cachedData); err == nil {
			cachedData["from_cache"] = true
			return cachedData, nil
		}
	}

	// 2. Call phimapi
	responseBody, err := MakeAnonymousRequest(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch category movies: %w", err)
	}

	// 3. Parse raw JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(responseBody, &raw); err != nil {
		return map[string]interface{}{
			"success":  false,
			"error":    "Failed to parse API response",
			"raw_data": string(responseBody),
		}, nil
	}

	// 4. Chuẩn hoá về dạng data.items
	normalized, ok := utils.IsValidApiResponse(raw)
	if !ok {
		return map[string]interface{}{
			"success": false,
			"error":   "Unexpected response structure",
		}, nil
	}
	// 5. Add metadata
	normalized["from_cache"] = false
	normalized["request_url"] = targetURL
	normalized["timestamp"] = time.Now().Unix()

	// 6. Cache
	jsonStr, _ := json.Marshal(normalized)
	_ = rdb.Set(ctx, cacheKey, jsonStr, 16*time.Hour).Err()

	tagKey := "movie_category:cached_keys"
	ttl := 16 * time.Hour

	pipe := rdb.Pipeline()
	pipe.Set(ctx, cacheKey, jsonStr, ttl)
	pipe.SAdd(ctx, tagKey, cacheKey)
	pipe.Expire(ctx, tagKey, ttl)
	_, _ = pipe.Exec(ctx)

	return normalized, nil
}

// buildCategoryURL constructs the phimapi.com URL for category browsing
func buildCategoryURL(req lib.MoviesByCategoryRequest) string {
	baseURL := fmt.Sprintf("https://phimapi.com/v1/api/the-loai/%s", req.Category)
	params := url.Values{}
	params.Add("page", fmt.Sprintf("%d", req.Page))
	params.Add("sort_field", req.SortField)
	params.Add("sort_type", req.SortType)
	params.Add("limit", fmt.Sprintf("%d", req.Limit))

	if req.SortLang != "" {
		params.Add("sort_lang", req.SortLang)
	}
	if req.Country != "" {
		params.Add("country", req.Country)
	}
	if req.Year != 0 {
		params.Add("year", fmt.Sprintf("%d", req.Year))
	}
	return baseURL + "?" + params.Encode()
}

// buildCategoryCacheKey builds a Redis cache key for category movie list
func buildCategoryCacheKey(req lib.MoviesByCategoryRequest) string {
	return fmt.Sprintf("movie_category:%s:%d:%s:%s:%s:%s:%d:%d",
		req.Category, req.Page, req.SortField, req.SortType,
		req.SortLang, req.Country, req.Year, req.Limit)
}

// ---------COUNTRY SIDE SERVICES---------------//
func ListMoviesByCountry(req lib.MoviesByCategoryRequest) (map[string]interface{}, error) {
	rdb := config.RDB
	ctx := config.Ctx

	targetURL := buildCountryURL(req)
	cacheKey := buildCountryCacheKey(req)

	// 1. Redis cache
	if cached, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		var cachedData map[string]interface{}
		if err := json.Unmarshal([]byte(cached), &cachedData); err == nil {
			cachedData["from_cache"] = true
			return cachedData, nil
		}
	}

	// 2. Call remote API
	responseBody, err := MakeAnonymousRequest(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch country movies: %w", err)
	}

	// 3. Parse raw response
	var raw map[string]interface{}
	if err := json.Unmarshal(responseBody, &raw); err != nil {
		return map[string]interface{}{
			"success":  false,
			"error":    "Failed to parse API response",
			"raw_data": string(responseBody),
		}, nil
	}

	// 4. Normalize response to `data.items`
	normalized, ok := utils.IsValidApiResponse(raw)
	if !ok {
		return map[string]interface{}{
			"success": false,
			"error":   "Unexpected response structure",
		}, nil
	}

	// 5. Add metadata
	normalized["from_cache"] = false
	normalized["timestamp"] = time.Now().Unix()
	normalized["request_url"] = targetURL

	// 6. Cache it
	if jsonStr, err := json.Marshal(normalized); err == nil {
		tagKey := "movie_country:cached_keys"
		ttl := 16 * time.Hour

		pipe := rdb.Pipeline()
		pipe.Set(ctx, cacheKey, jsonStr, ttl)
		pipe.SAdd(ctx, tagKey, cacheKey)
		pipe.Expire(ctx, tagKey, ttl)
		_, _ = pipe.Exec(ctx)
	}

	return normalized, nil
}

func buildCountryURL(req lib.MoviesByCategoryRequest) string {
	baseURL := fmt.Sprintf("https://phimapi.com/v1/api/quoc-gia/%s", req.Country)
	params := url.Values{}
	params.Add("page", fmt.Sprintf("%d", req.Page))
	params.Add("sort_field", req.SortField)
	params.Add("sort_type", req.SortType)
	params.Add("limit", fmt.Sprintf("%d", req.Limit))

	if req.SortLang != "" {
		params.Add("sort_lang", req.SortLang)
	}
	if req.Country != "" {
		params.Add("country", req.Country)
	}
	if req.Year != 0 {
		params.Add("year", fmt.Sprintf("%d", req.Year))
	}
	return baseURL + "?" + params.Encode()
}

func buildCountryCacheKey(req lib.MoviesByCategoryRequest) string {
	return fmt.Sprintf("movie_country:%s:%d:%s:%s:%s:%s:%d:%d",
		req.Category, req.Page, req.SortField, req.SortType,
		req.SortLang, req.Country, req.Year, req.Limit)
}
