package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/t12e/takealot-cli/internal/models"
)

const (
	SearchAPIBase = "https://api.takealot.com/rest/v-1-14-0"
	MobileAPIBase = "https://api.takealot.com/rest/v-1-16-0"
)

type Client struct {
	httpClient *http.Client
	searchBase string
	mobileBase string
	imageHosts map[string]struct{}
}

func New() *Client {
	return NewWithHTTPClient(http.DefaultClient, SearchAPIBase, MobileAPIBase)
}

// NewWithHTTPClient is useful for deterministic tests and local API proxies.
func NewWithHTTPClient(httpClient *http.Client, searchBase, mobileBase string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	imageHosts := map[string]struct{}{"media.takealot.com": {}}
	if parsed, err := url.Parse(mobileBase); err == nil && parsed.Hostname() != "" {
		imageHosts[strings.ToLower(parsed.Hostname())] = struct{}{}
	}
	return &Client{
		httpClient: httpClient,
		searchBase: strings.TrimRight(searchBase, "/"),
		mobileBase: strings.TrimRight(mobileBase, "/"),
		imageHosts: imageHosts,
	}
}

type APIError struct {
	StatusCode int
	Code       string
	Message    string
	RetryAfter string
}

func (e *APIError) Error() string {
	if e.RetryAfter != "" {
		return fmt.Sprintf("takealot API error (%s, HTTP %d): %s; retry-after: %s", e.Code, e.StatusCode, e.Message, e.RetryAfter)
	}
	return fmt.Sprintf("takealot API error (%s, HTTP %d): %s", e.Code, e.StatusCode, e.Message)
}

var plidPattern = regexp.MustCompile(`(?i)(?:^|/)PLID([0-9]+)(?:/|$)`)

// ParsePLID accepts a numeric PLID, PLID123, or a Takealot product URL.
// Product IDs and TSINs are deliberately not accepted because they are
// separate identifiers and cannot safely be inferred from an arbitrary number.
func ParsePLID(reference string) (string, error) {
	ref := strings.TrimSpace(reference)
	if ref == "" {
		return "", errors.New("invalid product reference: value is empty")
	}
	if digitsOnly(ref) {
		return ref, nil
	}
	if match := regexp.MustCompile(`(?i)^PLID([0-9]+)$`).FindStringSubmatch(ref); len(match) == 2 {
		return match[1], nil
	}

	u, err := url.Parse(ref)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid product reference %q: expected a numeric PLID or Takealot product URL", reference)
	}
	host := strings.ToLower(u.Hostname())
	if host != "takealot.com" && host != "www.takealot.com" {
		return "", fmt.Errorf("invalid product reference %q: URL host must be takealot.com", reference)
	}
	if match := plidPattern.FindStringSubmatch(u.Path); len(match) == 2 {
		return match[1], nil
	}
	return "", fmt.Errorf("invalid product reference %q: URL must end with a PLID#### path segment", reference)
}

func digitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (c *Client) Search(ctx context.Context, query string, limit int) (models.SearchResponse, error) {
	if strings.TrimSpace(query) == "" {
		return models.SearchResponse{}, errors.New("search query cannot be empty")
	}
	if limit <= 0 {
		return models.SearchResponse{}, errors.New("search limit must be greater than zero")
	}
	values := url.Values{}
	values.Set("r", "1")
	values.Set("sb", "1")
	values.Set("si", "63b04484becf69dd89948104f99effc7")
	values.Set("qsearch", query)
	values.Set("searchbox", "true")

	var raw map[string]any
	if err := c.getJSON(ctx, c.searchBase, "/searches/products,filters,facets,sort_options,breadcrumbs,slots_audience,context,seo,layout", values, true, &raw); err != nil {
		return models.SearchResponse{}, err
	}

	results := make([]models.ProductSummary, 0)
	sections := asMap(raw["sections"])
	products := asMap(sections["products"])
	for _, result := range asSlice(products["results"]) {
		resultMap := asMap(result)
		view := asMap(resultMap["product_views"])
		product := normalizeSearchProduct(view, resultMap)
		if product.PLID == "" {
			continue
		}
		results = append(results, product)
		if len(results) == limit {
			break
		}
	}
	return models.SearchResponse{Query: query, Returned: len(results), Results: results}, nil
}

func (c *Client) GetProduct(ctx context.Context, reference string) (models.ProductDetails, error) {
	plid, err := ParsePLID(reference)
	if err != nil {
		return models.ProductDetails{}, err
	}
	values := url.Values{}
	values.Set("platform", "android")
	values.Set("show_takealot_now_alt", "false")
	values.Set("offer_opt", "true")
	var raw map[string]any
	if err := c.getJSON(ctx, c.mobileBase, "/product-details/PLID"+plid, values, false, &raw); err != nil {
		return models.ProductDetails{}, err
	}
	return normalizeProductDetails(raw, plid), nil
}

type ImageDownloadOptions struct {
	Limit     int
	Directory string
}

func (c *Client) DownloadProductImages(ctx context.Context, reference string, options ImageDownloadOptions) (models.ProductImagesResponse, error) {
	plid, err := ParsePLID(reference)
	if err != nil {
		return models.ProductImagesResponse{}, err
	}
	if options.Limit == 0 {
		options.Limit = 1
	}
	if options.Limit < 0 {
		return models.ProductImagesResponse{}, errors.New("image limit must be greater than zero")
	}
	if options.Limit > 10 {
		return models.ProductImagesResponse{}, errors.New("image limit cannot be greater than 10")
	}

	product, err := c.GetProduct(ctx, plid)
	if err != nil {
		return models.ProductImagesResponse{}, err
	}
	usingDefaultDirectory := options.Directory == ""
	directory := options.Directory
	if directory == "" {
		directory, err = defaultImageDirectory(plid)
		if err != nil {
			return models.ProductImagesResponse{}, err
		}
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return models.ProductImagesResponse{}, fmt.Errorf("resolve image directory: %w", err)
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return models.ProductImagesResponse{}, fmt.Errorf("create image directory: %w", err)
	}
	if usingDefaultDirectory {
		defaultDirectories := []string{filepath.Dir(filepath.Dir(directory)), filepath.Dir(directory), directory}
		for _, hiddenDirectory := range defaultDirectories {
			if err := markHidden(hiddenDirectory); err != nil {
				return models.ProductImagesResponse{}, fmt.Errorf("hide image directory: %w", err)
			}
		}
	}

	limit := options.Limit
	if limit > len(product.Gallery) {
		limit = len(product.Gallery)
	}
	result := models.ProductImagesResponse{
		PLID:      plid,
		Title:     product.Title,
		Directory: directory,
		Images:    make([]models.DownloadedImage, 0, limit),
	}
	for index, sourceURL := range product.Gallery[:limit] {
		image, err := c.downloadImage(ctx, sourceURL, directory, index+1)
		if err != nil {
			return models.ProductImagesResponse{}, fmt.Errorf("download product image %d: %w", index+1, err)
		}
		result.Images = append(result.Images, image)
	}
	return result, nil
}

func defaultImageDirectory(plid string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".takealot", "images", plid), nil
}

func (c *Client) downloadImage(ctx context.Context, sourceURL, directory string, index int) (models.DownloadedImage, error) {
	parsed, err := url.Parse(sourceURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" {
		return models.DownloadedImage{}, fmt.Errorf("invalid image URL %q", sourceURL)
	}
	if _, ok := c.imageHosts[strings.ToLower(parsed.Hostname())]; !ok {
		return models.DownloadedImage{}, fmt.Errorf("image host %q is not allowed", parsed.Hostname())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return models.DownloadedImage{}, fmt.Errorf("build image request: %w", err)
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Referer", "https://www.takealot.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; takealot-cli/1.0)")
	response, err := c.httpClient.Do(req)
	if err != nil {
		return models.DownloadedImage{}, fmt.Errorf("request image: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20+1))
	if err != nil {
		return models.DownloadedImage{}, fmt.Errorf("read image response: %w", err)
	}
	if isCloudflareChallenge(response, body) {
		return models.DownloadedImage{}, errors.New("Takealot returned a Cloudflare challenge for the image")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return models.DownloadedImage{}, fmt.Errorf("image request returned HTTP %d", response.StatusCode)
	}
	if len(body) == 0 {
		return models.DownloadedImage{}, errors.New("image response was empty")
	}
	if len(body) > 16<<20 {
		return models.DownloadedImage{}, errors.New("image response exceeded 16 MiB limit")
	}

	contentType := response.Header.Get("Content-Type")
	if parsedType, _, parseErr := mime.ParseMediaType(contentType); parseErr == nil {
		contentType = parsedType
	}
	if !strings.HasPrefix(contentType, "image/") {
		contentType = http.DetectContentType(body)
	}
	if !strings.HasPrefix(contentType, "image/") {
		return models.DownloadedImage{}, fmt.Errorf("response is not an image (content type %q)", contentType)
	}

	filename := fmt.Sprintf("%02d%s", index, imageExtension(contentType))
	path := filepath.Join(directory, filename)
	temporary, err := os.CreateTemp(directory, ".image-*")
	if err != nil {
		return models.DownloadedImage{}, fmt.Errorf("create temporary image file: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = os.Remove(temporaryName)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return models.DownloadedImage{}, fmt.Errorf("set image permissions: %w", err)
	}
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return models.DownloadedImage{}, fmt.Errorf("write image: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return models.DownloadedImage{}, fmt.Errorf("close image: %w", err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return models.DownloadedImage{}, fmt.Errorf("replace existing image: %w", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return models.DownloadedImage{}, fmt.Errorf("store image: %w", err)
	}
	return models.DownloadedImage{Index: index, SourceURL: sourceURL, LocalPath: path, ContentType: contentType, Bytes: int64(len(body))}, nil
}

func imageExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/avif":
		return ".avif"
	case "image/svg+xml":
		return ".svg"
	default:
		return ".img"
	}
}

type ReviewOptions struct {
	Rating  int
	Sort    string
	Page    int
	Variant string
}

func (c *Client) GetReviews(ctx context.Context, reference string, options ReviewOptions) (models.ReviewsResponse, error) {
	plid, err := ParsePLID(reference)
	if err != nil {
		return models.ReviewsResponse{}, err
	}
	if options.Rating < 0 || options.Rating > 5 {
		return models.ReviewsResponse{}, errors.New("rating must be between 1 and 5")
	}
	if options.Rating == 0 {
		// zero means no filter; negative values were rejected above.
	}
	if options.Sort != "" && options.Sort != "helpful" && options.Sort != "latest" {
		return models.ReviewsResponse{}, errors.New("sort must be helpful or latest")
	}
	if options.Page < 0 {
		return models.ReviewsResponse{}, errors.New("page must be zero or greater")
	}

	values := url.Values{}
	if options.Rating > 0 {
		values.Set("rating", strconv.Itoa(options.Rating))
	}
	if options.Sort == "latest" {
		values.Set("sort", "SO_LATEST")
	}
	if options.Page > 0 {
		values.Set("page", strconv.Itoa(options.Page))
	}
	if strings.TrimSpace(options.Variant) != "" {
		values.Set("colour_variant", options.Variant)
	}

	var raw map[string]any
	if err := c.getJSON(ctx, c.mobileBase, "/product-reviews/plid/"+plid, values, false, &raw); err != nil {
		return models.ReviewsResponse{}, err
	}
	return normalizeReviews(raw, plid), nil
}

func (c *Client) getJSON(ctx context.Context, base, path string, query url.Values, search bool, destination any) error {
	u, err := url.Parse(strings.TrimRight(base, "/") + path)
	if err != nil {
		return fmt.Errorf("build request URL: %w", err)
	}
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://www.takealot.com")
	req.Header.Set("Referer", "https://www.takealot.com/")
	if search {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Mobile Safari/537.36")
	} else {
		req.Header.Set("User-Agent", "TAL-Android/3.51.0 (fi.android.takealot; build:800735; 14; samsung; SM-S928B; Phone)")
	}

	response, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("takealot request failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("read takealot response: %w", err)
	}
	if isCloudflareChallenge(response, body) {
		return apiError(response, body)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return apiError(response, body)
	}
	if err := json.Unmarshal(body, destination); err != nil {
		return fmt.Errorf("decode takealot response: malformed JSON: %w", err)
	}
	return nil
}

func apiError(response *http.Response, body []byte) error {
	message := strings.TrimSpace(string(body))
	if len(message) > 300 {
		message = message[:300] + "..."
	}
	code := "http_error"
	if isCloudflareChallenge(response, body) {
		code = "cloudflare_challenge"
		message = "Takealot returned a Cloudflare challenge; automated catalogue access is temporarily unavailable"
	} else if response.StatusCode == http.StatusForbidden {
		code = "forbidden"
	} else if response.StatusCode == http.StatusNotFound {
		code = "not_found"
	} else if response.StatusCode == http.StatusTooManyRequests {
		code = "rate_limited"
	}
	if message == "" {
		message = http.StatusText(response.StatusCode)
	}
	return &APIError{StatusCode: response.StatusCode, Code: code, Message: message, RetryAfter: response.Header.Get("Retry-After")}
}

func isCloudflareChallenge(response *http.Response, body []byte) bool {
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "just a moment") || strings.Contains(lower, "challenge-platform") || strings.Contains(lower, "performing security verification") || response.Header.Get("cf-mitigated") != ""
}

func normalizeSearchProduct(view, result map[string]any) models.ProductSummary {
	core := asMap(view["core"])
	buybox := asMap(view["buybox_summary"])
	rating := normalizeRating(asMap(view["review_summary"]))
	stock := normalizeStock(asMap(view["stock_availability_summary"]))
	gallery := extractImageURLs(view["gallery"])
	if len(gallery) == 0 {
		gallery = extractImageURLs(result["gallery"])
	}
	plid := getIntString(core["id"])
	return models.ProductSummary{
		PLID:         plid,
		ProductID:    getInt(buybox["product_id"]),
		TSIN:         getInt(buybox["tsin"]),
		Title:        firstString(core["title"], result["title"]),
		Subtitle:     firstString(core["subtitle"], result["subtitle"]),
		Brand:        firstString(core["brand"], result["brand"]),
		URL:          productURLWithSlug(firstString(core["desktop_href"], result["desktop_href"], result["href"]), plid, firstString(core["slug"])),
		PriceDisplay: firstString(buybox["pretty_price"], buybox["price"], view["price"]),
		Prices:       getIntSlice(buybox["prices"]),
		Rating:       rating,
		Stock:        stock,
		ImageURLs:    gallery,
	}
}

func normalizeProductDetails(raw map[string]any, plid string) models.ProductDetails {
	core := asMap(raw["core"])
	buybox := asMap(raw["buybox"])
	buyboxItem := selectedBuyboxItem(buybox)
	reviews := asMap(raw["reviews"])
	prices := getIntSlice(buyboxItem["prices"])
	if len(prices) == 0 {
		if price := getInt(buyboxItem["price"]); price != 0 {
			prices = []int64{price}
		}
	}
	eventProduct := asMap(asMap(asMap(raw["event_data"])["documents"])["product"])
	stock := normalizeStock(eventProduct)
	if stock.InStock == nil {
		stock = normalizeStock(buyboxItem)
	}
	summary := models.ProductSummary{
		PLID:         plid,
		ProductID:    firstInt(buybox["product_id"], buyboxItem["product_id"], buybox["id"]),
		TSIN:         firstInt(buybox["tsin"], buyboxItem["tsin"]),
		Title:        firstString(raw["title"], core["title"]),
		Subtitle:     firstString(raw["subtitle"], core["subtitle"]),
		Brand:        firstString(core["brand"], raw["brand"]),
		URL:          productURL(firstString(raw["desktop_href"], core["desktop_href"]), plid),
		PriceDisplay: firstString(buyboxItem["pretty_price"], buybox["pretty_price"], buyboxItem["price"], buybox["price"]),
		Prices:       prices,
		Rating:       normalizeRating(reviews),
		Stock:        stock,
	}

	descriptionMap := asMap(raw["description"])
	descriptionHTML := firstString(descriptionMap["html"], descriptionMap["content_html"])
	description := firstString(descriptionMap["text"], descriptionMap["displayable_text"], descriptionMap["copy"])
	if description == "" {
		description = firstString(raw["description"])
	}
	if description == "" {
		description = descriptionHTML
	}
	description = htmlToText(description)

	return models.ProductDetails{
		ProductSummary:  summary,
		Description:     description,
		DescriptionHTML: descriptionHTML,
		BulletPoints:    normalizeBulletPoints(raw["bullet_point_attributes"]),
		Attributes:      normalizeAttributes(raw["product_information"]),
		Variants:        normalizeVariants(raw["variants"]),
		Seller:          normalizeSeller(firstMap(buybox["seller_detail"], buyboxItem["seller_detail"], buybox["seller"], buyboxItem["seller"])),
		Returns:         normalizeReturns(raw["exchanges_and_returns"]),
		Gallery:         extractImageURLs(raw["gallery"]),
	}
}

func selectedBuyboxItem(buybox map[string]any) map[string]any {
	for _, item := range asSlice(buybox["items"]) {
		value := asMap(item)
		if value != nil && getBool(value["is_selected"]) {
			return value
		}
	}
	if items := asSlice(buybox["items"]); len(items) > 0 {
		return asMap(items[0])
	}
	return buybox
}

func normalizeReviews(raw map[string]any, plid string) models.ReviewsResponse {
	page := asMap(raw["page_info"])
	result := models.ReviewsResponse{
		PLID: plid,
		Page: models.PageInfo{
			Total:       int(getInt(page["total"])),
			TotalPages:  int(getInt(page["total_pages"])),
			CurrentPage: int(getInt(page["current_page"])),
			PageSize:    int(getInt(page["page_size"])),
		},
		Reviews: make([]models.Review, 0),
	}
	for _, item := range asSlice(raw["reviews"]) {
		review := asMap(item)
		text := asMap(review["text"])
		result.Reviews = append(result.Reviews, models.Review{
			Rating:            int(getInt(review["rating"])),
			Text:              firstString(text["body"], review["body"], review["text"]),
			Date:              firstString(review["date"], review["created_at"]),
			Upvotes:           int(firstInt(review["num_upvotes"], review["upvotes"])),
			VariantInfo:       asMap(review["variant_info"]),
			TimeAfterPurchase: firstString(review["time_after_purchase"]),
		})
	}
	for _, item := range asSlice(raw["sort_options"]) {
		option := asMap(item)
		result.SortOptions = append(result.SortOptions, models.SortOption{Value: firstString(option["value"], option["id"]), Label: firstString(option["label"], option["title"], option["display_value"]), Selected: getBool(firstValue(option, "selected", "is_selected"))})
	}
	for _, item := range asSlice(raw["filters"]) {
		filter := asMap(item)
		options := make([]string, 0)
		filterOptions := asSlice(filter["options"])
		if len(filterOptions) == 0 {
			filterOptions = asSlice(filter["entries"])
		}
		for _, option := range filterOptions {
			if optionMap := asMap(option); optionMap != nil {
				options = append(options, firstString(optionMap["value"], optionMap["label"], optionMap["name"], optionMap["display_value"]))
			} else if value := firstString(option); value != "" {
				options = append(options, value)
			}
		}
		result.Filters = append(result.Filters, models.ReviewFilter{Type: firstString(filter["type"], filter["id"], filter["filter_name"]), Title: firstString(filter["title"], filter["label"], filter["display_name"]), Options: options})
	}
	return result
}

func normalizeRating(raw map[string]any) models.RatingSummary {
	return models.RatingSummary{Average: getFloat(firstValue(raw, "star_rating", "average", "rating")), Count: int(firstInt(raw["review_count"], raw["count"], raw["total"])), Distribution: normalizeDistribution(raw["distribution"])}
}

func normalizeDistribution(raw any) models.RatingDistribution {
	result := models.RatingDistribution{}
	if values := asMap(raw); values != nil {
		result.OneStar = int(getInt(firstValue(values, "1", "one", "one_star", "num_1_star_ratings")))
		result.TwoStar = int(getInt(firstValue(values, "2", "two", "two_star", "num_2_star_ratings")))
		result.ThreeStar = int(getInt(firstValue(values, "3", "three", "three_star", "num_3_star_ratings")))
		result.FourStar = int(getInt(firstValue(values, "4", "four", "four_star", "num_4_star_ratings")))
		result.FiveStar = int(getInt(firstValue(values, "5", "five", "five_star", "num_5_star_ratings")))
	}
	for _, item := range asSlice(raw) {
		entry := asMap(item)
		if entry == nil {
			continue
		}
		count := firstInt(entry["count"], entry["total"])
		switch getInt(firstValue(entry, "rating", "stars", "value")) {
		case 1:
			result.OneStar = int(count)
		case 2:
			result.TwoStar = int(count)
		case 3:
			result.ThreeStar = int(count)
		case 4:
			result.FourStar = int(count)
		case 5:
			result.FiveStar = int(count)
		}
	}
	return result
}

func normalizeStock(raw map[string]any) models.StockSummary {
	if raw == nil {
		return models.StockSummary{}
	}
	var inStock *bool
	if value, ok := firstExisting(raw, "is_in_stock", "in_stock", "is_available", "available", "is_add_to_cart_available"); ok {
		parsed := getBool(value)
		inStock = &parsed
	}
	status := firstString(raw["status"], raw["displayable_text"], raw["availability"])
	if status == "" && inStock != nil {
		if *inStock {
			status = "In stock"
		} else {
			status = "Unavailable"
		}
	}
	return models.StockSummary{
		Status:              status,
		InStock:             inStock,
		LeadTime:            getBool(raw["lead_time"]),
		EstimatedDelivery:   firstString(raw["estimated_delivery"], raw["delivery_date"]),
		DistributionCentres: getStringSlice(raw["distribution_centres"]),
	}
}

func normalizeBulletPoints(raw any) []models.BulletPoint {
	container := asMap(raw)
	items := asSlice(container["items"])
	result := make([]models.BulletPoint, 0, len(items))
	for _, item := range items {
		value := asMap(item)
		if value == nil {
			continue
		}
		text := firstString(value["description"], value["text"], value["displayable_text"])
		if text != "" {
			positive, ok := firstBool(value, "positive")
			point := models.BulletPoint{Text: text, Type: firstString(value["type"])}
			if ok {
				point.Positive = &positive
			}
			result = append(result, point)
		}
	}
	return result
}

func normalizeAttributes(raw any) []models.Attribute {
	container := asMap(raw)
	items := asSlice(container["items"])
	result := make([]models.Attribute, 0, len(items))
	for _, item := range items {
		value := asMap(item)
		if value == nil {
			continue
		}
		result = append(result, models.Attribute{Name: firstString(value["display_name"], value["name"], value["label"]), DisplayText: firstString(value["displayable_text"], value["text"]), Value: value["value"], ItemType: firstString(value["item_type"], value["type"])})
	}
	return result
}

func normalizeVariants(raw any) []models.VariantSelector {
	container := asMap(raw)
	result := make([]models.VariantSelector, 0)
	for _, item := range asSlice(container["selectors"]) {
		selector := asMap(item)
		if selector == nil {
			continue
		}
		variant := models.VariantSelector{Type: firstString(selector["selector_type"], selector["type"], selector["action"]), Title: firstString(selector["title"], selector["call_to_action"]), Options: make([]models.VariantOption, 0)}
		for _, optionItem := range asSlice(selector["options"]) {
			option := asMap(optionItem)
			if option == nil {
				continue
			}
			value := asMap(option["value"])
			variant.Options = append(variant.Options, models.VariantOption{ID: firstString(option["id"]), Name: firstString(value["name"], option["name"], option["label"]), Value: firstString(value["value"], option["value"]), Enabled: getBool(option["is_enabled"]), Selected: getBool(option["is_selected"]), URL: firstString(option["href"], option["desktop_href"]), ImageURLs: extractImageURLs(option["image"])})
		}
		result = append(result, variant)
	}
	return result
}

func normalizeSeller(raw map[string]any) *models.Seller {
	if raw == nil {
		return nil
	}
	result := &models.Seller{Name: firstString(raw["name"], raw["display_name"], raw["seller_name"]), ID: firstInt(raw["seller_id"], raw["id"])}
	if value, ok := firstExisting(raw, "fulfilled_by_takealot", "is_fulfilled_by_takealot"); ok {
		parsed := getBool(value)
		result.FulfilledByTakealot = &parsed
	}
	if result.Name == "" && result.ID == 0 && result.FulfilledByTakealot == nil {
		return nil
	}
	return result
}

func normalizeReturns(raw any) *models.Returns {
	value := asMap(raw)
	if value == nil {
		return nil
	}
	copyValue := firstString(value["copy"], value["text"], value["description"])
	result := &models.Returns{}
	if strings.HasPrefix(copyValue, "http://") || strings.HasPrefix(copyValue, "https://") {
		result.URL = copyValue
	} else {
		result.Text = copyValue
	}
	if result.URL == "" && result.Text == "" {
		return nil
	}
	return result
}

func productURL(value, plid string) string {
	return productURLWithSlug(value, plid, "")
}

func productURLWithSlug(value, plid, slug string) string {
	if value != "" {
		if strings.HasPrefix(value, "/") {
			value = "https://www.takealot.com" + value
		}
		return canonicalProductURL(value)
	}
	if slug != "" {
		return "https://www.takealot.com/" + url.PathEscape(slug) + "/PLID" + plid
	}
	return "https://www.takealot.com/PLID" + plid
}

func canonicalProductURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return value
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "takealot.com" && host != "www.takealot.com" {
		return value
	}
	segments := strings.Split(parsed.Path, "/")
	if len(segments) > 1 && strings.EqualFold(segments[1], "product") {
		segments = append(segments[:1], segments[2:]...)
		parsed.Path = strings.Join(segments, "/")
	}
	return parsed.String()
}

func asMap(value any) map[string]any {
	if result, ok := value.(map[string]any); ok {
		return result
	}
	return nil
}

func asSlice(value any) []any {
	if result, ok := value.([]any); ok {
		return result
	}
	return nil
}

func firstMap(values ...any) map[string]any {
	for _, value := range values {
		if result := asMap(value); result != nil {
			return result
		}
	}
	return nil
}

func firstValue(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func firstExisting(values map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func firstString(values ...any) string {
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return typed
			}
		case fmt.Stringer:
			if text := typed.String(); text != "" {
				return text
			}
		}
	}
	return ""
}

func getInt(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(strings.ReplaceAll(typed, ",", "")), 10, 64)
		return parsed
	default:
		return 0
	}
}

func firstInt(values ...any) int64 {
	for _, value := range values {
		if parsed := getInt(value); parsed != 0 {
			return parsed
		}
	}
	return 0
}

func getIntString(value any) string {
	if valueString := firstString(value); valueString != "" {
		return valueString
	}
	if parsed := getInt(value); parsed != 0 {
		return strconv.FormatInt(parsed, 10)
	}
	return ""
}

func getFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	default:
		return float64(getInt(value))
	}
}

func getBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(typed))
		return parsed
	default:
		return false
	}
}

func firstBool(values map[string]any, keys ...string) (bool, bool) {
	value, ok := firstExisting(values, keys...)
	return getBool(value), ok
}

func getIntSlice(value any) []int64 {
	result := make([]int64, 0)
	for _, item := range asSlice(value) {
		if parsed := getInt(item); parsed != 0 {
			result = append(result, parsed)
		}
	}
	return result
}

func getStringSlice(value any) []string {
	result := make([]string, 0)
	for _, item := range asSlice(value) {
		if text := firstString(item); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func extractImageURLs(value any) []string {
	result := make([]string, 0)
	seen := make(map[string]struct{})
	var visit func(any, string)
	visit = func(current any, key string) {
		switch typed := current.(type) {
		case string:
			if isImageURL(typed, key) {
				resolved := strings.ReplaceAll(typed, "{size}", "full")
				if _, ok := seen[resolved]; !ok {
					seen[resolved] = struct{}{}
					result = append(result, resolved)
				}
			}
		case []any:
			for _, item := range typed {
				visit(item, key)
			}
		case map[string]any:
			for childKey, item := range typed {
				visit(item, childKey)
			}
		}
	}
	visit(value, "")
	return result
}

func isImageURL(value, key string) bool {
	lower := strings.ToLower(value)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}
	return strings.Contains(lower, "media.takealot") || strings.Contains(lower, ".jpg") || strings.Contains(lower, ".jpeg") || strings.Contains(lower, ".png") || strings.Contains(lower, ".webp") || strings.Contains(strings.ToLower(key), "image")
}

var (
	htmlBreakPattern = regexp.MustCompile(`(?i)</?(?:br|p|li|h[1-6]|div|tr)[^>]*>`)
	htmlTagPattern   = regexp.MustCompile(`(?s)<[^>]*>`)
)

func htmlToText(value string) string {
	if !strings.Contains(value, "<") {
		return strings.TrimSpace(value)
	}
	value = htmlBreakPattern.ReplaceAllString(value, "\n")
	value = htmlTagPattern.ReplaceAllString(value, "")
	value = html.UnescapeString(value)
	lines := make([]string, 0)
	for _, line := range strings.Split(value, "\n") {
		if text := strings.Join(strings.Fields(line), " "); text != "" {
			lines = append(lines, text)
		}
	}
	return strings.Join(lines, "\n")
}
