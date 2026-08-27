package models

// SearchResponse is the stable public shape returned by the search command.
type SearchResponse struct {
	Query    string           `json:"query"`
	Returned int              `json:"returned"`
	Results  []ProductSummary `json:"results"`
}

type ProductSummary struct {
	PLID         string        `json:"plid"`
	ProductID    int64         `json:"product_id,omitempty"`
	TSIN         int64         `json:"tsin,omitempty"`
	Title        string        `json:"title"`
	Subtitle     string        `json:"subtitle,omitempty"`
	Brand        string        `json:"brand,omitempty"`
	URL          string        `json:"url"`
	PriceDisplay string        `json:"price_display,omitempty"`
	Prices       []int64       `json:"prices,omitempty"`
	Rating       RatingSummary `json:"rating"`
	Stock        StockSummary  `json:"stock"`
	ImageURLs    []string      `json:"image_urls,omitempty"`
}

type RatingSummary struct {
	Average      float64            `json:"average"`
	Count        int                `json:"count"`
	Distribution RatingDistribution `json:"distribution"`
}

type RatingDistribution struct {
	OneStar   int `json:"1"`
	TwoStar   int `json:"2"`
	ThreeStar int `json:"3"`
	FourStar  int `json:"4"`
	FiveStar  int `json:"5"`
}

type StockSummary struct {
	Status              string   `json:"status,omitempty"`
	InStock             *bool    `json:"in_stock,omitempty"`
	LeadTime            bool     `json:"lead_time,omitempty"`
	EstimatedDelivery   string   `json:"estimated_delivery,omitempty"`
	DistributionCentres []string `json:"distribution_centres,omitempty"`
}

type ProductDetails struct {
	ProductSummary
	Description     string            `json:"description,omitempty"`
	DescriptionHTML string            `json:"description_html,omitempty"`
	BulletPoints    []BulletPoint     `json:"bullet_points,omitempty"`
	Attributes      []Attribute       `json:"attributes,omitempty"`
	Variants        []VariantSelector `json:"variants,omitempty"`
	Seller          *Seller           `json:"seller,omitempty"`
	Returns         *Returns          `json:"returns,omitempty"`
	Gallery         []string          `json:"gallery"`
}

type ProductImagesResponse struct {
	PLID      string            `json:"plid"`
	Title     string            `json:"title,omitempty"`
	Directory string            `json:"directory"`
	Images    []DownloadedImage `json:"images"`
}

type DownloadedImage struct {
	Index       int    `json:"index"`
	SourceURL   string `json:"source_url"`
	LocalPath   string `json:"local_path"`
	ContentType string `json:"content_type"`
	Bytes       int64  `json:"bytes"`
}

type BulletPoint struct {
	Text     string `json:"text"`
	Positive *bool  `json:"positive,omitempty"`
	Type     string `json:"type,omitempty"`
}

type Attribute struct {
	Name        string `json:"name"`
	DisplayText string `json:"display_text,omitempty"`
	Value       any    `json:"value,omitempty"`
	ItemType    string `json:"item_type,omitempty"`
}

type VariantSelector struct {
	Type    string          `json:"type"`
	Title   string          `json:"title,omitempty"`
	Options []VariantOption `json:"options"`
}

type VariantOption struct {
	ID        string   `json:"id,omitempty"`
	Name      string   `json:"name"`
	Value     string   `json:"value,omitempty"`
	Enabled   bool     `json:"enabled"`
	Selected  bool     `json:"selected"`
	URL       string   `json:"url,omitempty"`
	ImageURLs []string `json:"image_urls,omitempty"`
}

type Seller struct {
	Name                string `json:"name,omitempty"`
	ID                  int64  `json:"id,omitempty"`
	FulfilledByTakealot *bool  `json:"fulfilled_by_takealot,omitempty"`
}

type Returns struct {
	Text string `json:"text,omitempty"`
	URL  string `json:"url,omitempty"`
}

type ReviewsResponse struct {
	PLID        string         `json:"plid"`
	Page        PageInfo       `json:"page"`
	SortOptions []SortOption   `json:"sort_options,omitempty"`
	Filters     []ReviewFilter `json:"filters,omitempty"`
	Reviews     []Review       `json:"reviews"`
}

type PageInfo struct {
	Total       int `json:"total"`
	TotalPages  int `json:"total_pages"`
	CurrentPage int `json:"current_page"`
	PageSize    int `json:"page_size"`
}

type SortOption struct {
	Value    string `json:"value"`
	Label    string `json:"label,omitempty"`
	Selected bool   `json:"selected,omitempty"`
}

type ReviewFilter struct {
	Type    string   `json:"type,omitempty"`
	Title   string   `json:"title,omitempty"`
	Options []string `json:"options,omitempty"`
}

type Review struct {
	Rating            int            `json:"rating"`
	Text              string         `json:"text"`
	Date              string         `json:"date,omitempty"`
	Upvotes           int            `json:"upvotes,omitempty"`
	VariantInfo       map[string]any `json:"variant_info,omitempty"`
	TimeAfterPurchase string         `json:"time_after_purchase,omitempty"`
}
