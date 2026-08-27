package models

import "time"

type AuthStatus struct {
	Authenticated         bool      `json:"authenticated"`
	CustomerID            string    `json:"customer_id,omitempty"`
	JWTExpiresAt          time.Time `json:"jwt_expires_at,omitempty"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at,omitempty"`
}

type WishlistResponse struct {
	CustomerID string     `json:"customer_id,omitempty"`
	Page       PageInfo   `json:"page"`
	Wishlists  []Wishlist `json:"wishlists"`
}

type Wishlist struct {
	GroupID   string         `json:"group_id"`
	Name      string         `json:"name,omitempty"`
	ItemCount int            `json:"item_count,omitempty"`
	Items     []WishlistItem `json:"items,omitempty"`
}

type WishlistItem struct {
	PLID      string `json:"plid,omitempty"`
	ProductID int64  `json:"product_id,omitempty"`
	TSIN      int64  `json:"tsin,omitempty"`
	SKU       int64  `json:"sku,omitempty"`
	Title     string `json:"title,omitempty"`
	URL       string `json:"url,omitempty"`
	ImageURL  string `json:"image_url,omitempty"`
}

type WishlistAction struct {
	Action    string          `json:"action"`
	GroupID   string          `json:"group_id,omitempty"`
	Product   *ProductSummary `json:"product,omitempty"`
	Completed bool            `json:"completed"`
}
