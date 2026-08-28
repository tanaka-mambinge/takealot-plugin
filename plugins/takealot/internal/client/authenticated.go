package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/t12e/takealot-cli/internal/auth"
	"github.com/t12e/takealot-cli/internal/config"
	"github.com/t12e/takealot-cli/internal/models"
)

type AuthenticatedClient struct {
	manager   *auth.Manager
	catalogue *Client
}

func NewAuthenticated() *AuthenticatedClient {
	return NewAuthenticatedWithHTTPClient(config.NewHTTPClient(), SearchAPIBase, MobileAPIBase, auth.NewStore())
}

func NewAuthenticatedWithHTTPClient(httpClient *http.Client, searchBase, mobileBase string, store *auth.Store) *AuthenticatedClient {
	if httpClient == nil {
		httpClient = config.NewHTTPClient()
	}
	return &AuthenticatedClient{
		manager:   auth.NewManager(httpClient, mobileBase, store),
		catalogue: NewWithHTTPClient(httpClient, searchBase, mobileBase),
	}
}

func (c *AuthenticatedClient) Login(ctx context.Context, email, password string, otp auth.OTPProvider) (models.AuthStatus, error) {
	session, err := c.manager.Login(ctx, email, password, otp)
	if err != nil {
		return models.AuthStatus{}, err
	}
	return authStatus(session), nil
}

func (c *AuthenticatedClient) Refresh(ctx context.Context) (models.AuthStatus, error) {
	session, err := c.manager.Refresh(ctx)
	if err != nil {
		return models.AuthStatus{}, err
	}
	return authStatus(session), nil
}

func (c *AuthenticatedClient) Status() (models.AuthStatus, error) {
	session, err := c.manager.Status()
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return models.AuthStatus{Authenticated: false}, nil
		}
		return models.AuthStatus{}, err
	}
	return authStatus(session), nil
}

func (c *AuthenticatedClient) Logout() error { return c.manager.Logout() }

func (c *AuthenticatedClient) ListWishlists(ctx context.Context, page, pageSize int) (models.WishlistResponse, error) {
	if page < 0 || pageSize < 0 {
		return models.WishlistResponse{}, errors.New("page and page-size cannot be negative")
	}
	customerID, err := c.customerID()
	if err != nil {
		return models.WishlistResponse{}, err
	}
	values := url.Values{}
	if page > 0 {
		values.Set("page", strconv.Itoa(page))
	}
	if pageSize > 0 {
		values.Set("page_size", strconv.Itoa(pageSize))
	}
	path := "/customers/" + url.PathEscape(customerID) + "/wishlists"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var raw map[string]any
	if err := c.manager.DoJSON(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return models.WishlistResponse{}, err
	}
	return normalizeWishlists(raw, customerID), nil
}

func (c *AuthenticatedClient) CreateWishlist(ctx context.Context, name string) (models.Wishlist, error) {
	if strings.TrimSpace(name) == "" {
		return models.Wishlist{}, errors.New("wishlist name cannot be empty")
	}
	customerID, err := c.customerID()
	if err != nil {
		return models.Wishlist{}, err
	}
	var raw map[string]any
	path := "/customers/" + url.PathEscape(customerID) + "/wishlists"
	if err := c.manager.DoJSON(ctx, http.MethodPost, path, map[string]any{"name": name}, &raw); err != nil {
		return models.Wishlist{}, err
	}
	return normalizeWishlist(firstMap(raw["wishlist"], raw["data"], raw)), nil
}

func (c *AuthenticatedClient) RenameWishlist(ctx context.Context, groupID, name string) (models.Wishlist, error) {
	if strings.TrimSpace(groupID) == "" || strings.TrimSpace(name) == "" {
		return models.Wishlist{}, errors.New("wishlist group-id and name are required")
	}
	customerID, err := c.customerID()
	if err != nil {
		return models.Wishlist{}, err
	}
	var raw map[string]any
	path := "/customers/" + url.PathEscape(customerID) + "/wishlists/" + url.PathEscape(groupID)
	if err := c.manager.DoJSON(ctx, http.MethodPut, path, map[string]any{"name": name}, &raw); err != nil {
		return models.Wishlist{}, err
	}
	return normalizeWishlist(firstMap(raw["wishlist"], raw["data"], raw)), nil
}

func (c *AuthenticatedClient) DeleteWishlist(ctx context.Context, groupID string) error {
	if strings.TrimSpace(groupID) == "" {
		return errors.New("wishlist group-id is required")
	}
	customerID, err := c.customerID()
	if err != nil {
		return err
	}
	path := "/customers/" + url.PathEscape(customerID) + "/wishlists/" + url.PathEscape(groupID)
	return c.manager.DoJSON(ctx, http.MethodDelete, path, nil, nil)
}

func (c *AuthenticatedClient) GetWishlistItems(ctx context.Context, groupID string, page, pageSize int) (models.Wishlist, error) {
	if strings.TrimSpace(groupID) == "" {
		return models.Wishlist{}, errors.New("wishlist group-id is required")
	}
	if page < 0 || pageSize < 0 {
		return models.Wishlist{}, errors.New("page and page-size cannot be negative")
	}
	customerID, err := c.customerID()
	if err != nil {
		return models.Wishlist{}, err
	}
	values := url.Values{}
	if page > 0 {
		values.Set("page", strconv.Itoa(page))
	}
	if pageSize > 0 {
		values.Set("page_size", strconv.Itoa(pageSize))
	}
	path := "/customers/" + url.PathEscape(customerID) + "/wishlists/" + url.PathEscape(groupID) + "/items"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var raw map[string]any
	if err := c.manager.DoJSON(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return models.Wishlist{}, err
	}
	result := normalizeWishlist(firstMap(raw["wishlist"], raw["data"], raw))
	result.GroupID = firstNonEmpty(result.GroupID, groupID)
	result.Items = normalizeWishlistItems(firstSlice(raw["items"], raw["products"], raw["data"]))
	return result, nil
}

// AddProductToWishlist uses the pid route used by the Android application. The
// product reference is resolved first so a PLID cannot be confused with a TSIN.
func (c *AuthenticatedClient) AddProductToWishlist(ctx context.Context, groupID, reference string) (models.WishlistAction, error) {
	if strings.TrimSpace(groupID) == "" {
		return models.WishlistAction{}, errors.New("wishlist group-id is required")
	}
	product, err := c.catalogue.GetProduct(ctx, reference)
	if err != nil {
		return models.WishlistAction{}, err
	}
	if product.ProductID == 0 {
		product, err = c.resolveWishlistProductID(ctx, product)
		if err != nil {
			return models.WishlistAction{}, err
		}
	}
	customerID, err := c.customerID()
	if err != nil {
		return models.WishlistAction{}, err
	}
	path := "/customers/" + url.PathEscape(customerID) + "/wishlists/items/pid/" + strconv.FormatInt(product.ProductID, 10)
	groupNumber, err := strconv.ParseInt(groupID, 10, 64)
	if err != nil || groupNumber <= 0 {
		return models.WishlistAction{}, errors.New("wishlist group-id must be a positive numeric ID")
	}
	body := map[string]any{"reset": false, "groups": []int64{groupNumber}}
	if err := c.manager.DoJSON(ctx, http.MethodPut, path, body, nil); err != nil {
		return models.WishlistAction{}, err
	}
	return models.WishlistAction{Action: "add", GroupID: groupID, Product: &product.ProductSummary, Completed: true}, nil
}

func (c *AuthenticatedClient) RemoveProductFromWishlists(ctx context.Context, reference string) (models.WishlistAction, error) {
	product, err := c.catalogue.GetProduct(ctx, reference)
	if err != nil {
		return models.WishlistAction{}, err
	}
	if product.ProductID == 0 {
		product, err = c.resolveWishlistProductID(ctx, product)
		if err != nil {
			return models.WishlistAction{}, err
		}
	}
	customerID, err := c.customerID()
	if err != nil {
		return models.WishlistAction{}, err
	}
	path := "/customers/" + url.PathEscape(customerID) + "/wishlists/items/pid/" + strconv.FormatInt(product.ProductID, 10)
	if err := c.manager.DoJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return models.WishlistAction{}, err
	}
	return models.WishlistAction{Action: "remove", Product: &product.ProductSummary, Completed: true}, nil
}

func (c *AuthenticatedClient) resolveWishlistProductID(ctx context.Context, product models.ProductDetails) (models.ProductDetails, error) {
	if strings.TrimSpace(product.Title) == "" {
		return models.ProductDetails{}, errors.New("product details did not contain a product ID required by the mobile wishlist API")
	}
	results, err := c.catalogue.Search(ctx, product.Title, 50)
	if err != nil {
		return models.ProductDetails{}, fmt.Errorf("resolve product ID through Takealot search: %w", err)
	}
	for _, candidate := range results.Results {
		if candidate.PLID == product.PLID && candidate.ProductID != 0 {
			product.ProductID = candidate.ProductID
			if product.TSIN == 0 {
				product.TSIN = candidate.TSIN
			}
			return product, nil
		}
	}
	return models.ProductDetails{}, errors.New("product details did not contain a product ID required by the mobile wishlist API, and Takealot search could not resolve it")
}

func (c *AuthenticatedClient) customerID() (string, error) {
	status, err := c.Status()
	if err != nil {
		return "", err
	}
	if !status.Authenticated || status.CustomerID == "" {
		return "", errors.New("not logged in; run takealot auth login first")
	}
	return status.CustomerID, nil
}

func authStatus(session auth.Session) models.AuthStatus {
	return models.AuthStatus{Authenticated: true, CustomerID: session.CustomerID, JWTExpiresAt: session.JWTExpiresAt, RefreshTokenExpiresAt: session.RefreshTokenExpiresAt}
}

func normalizeWishlists(raw map[string]any, customerID string) models.WishlistResponse {
	container := raw
	if nested := firstMap(raw["data"], raw["result"]); nested != nil {
		container = nested
	}
	page := asMap(container["page_info"])
	if page == nil {
		page = asMap(container["page"])
	}
	items := firstSlice(container["wishlists"], container["groups"], container["lists"], container["items"])
	result := models.WishlistResponse{CustomerID: customerID, Page: models.PageInfo{Total: int(getInt(page["total"])), TotalPages: int(getInt(page["total_pages"])), CurrentPage: int(getInt(page["current_page"])), PageSize: int(getInt(page["page_size"]))}, Wishlists: make([]models.Wishlist, 0, len(items))}
	for _, item := range items {
		if value := asMap(item); value != nil {
			result.Wishlists = append(result.Wishlists, normalizeWishlist(value))
		}
	}
	return result
}

func normalizeWishlist(raw map[string]any) models.Wishlist {
	if raw == nil {
		return models.Wishlist{}
	}
	items := firstSlice(raw["items"], raw["products"])
	return models.Wishlist{GroupID: getIntString(firstValue(raw, "group_id", "id", "groupId")), Name: firstString(raw["name"], raw["title"]), ItemCount: int(firstInt(raw["item_count"], raw["item_count_total"], raw["count"])), Items: normalizeWishlistItems(items)}
}

func normalizeWishlistItems(items []any) []models.WishlistItem {
	result := make([]models.WishlistItem, 0, len(items))
	for _, item := range items {
		value := asMap(item)
		if value == nil {
			continue
		}
		product := asMap(value["product"])
		if product == nil {
			product = value
		}
		result = append(result, models.WishlistItem{PLID: getIntString(firstValue(product, "plid", "plid_id")), ProductID: firstInt(product["product_id"], product["sku"]), TSIN: firstInt(product["tsin"], product["tsin_id"]), SKU: firstInt(product["sku"]), Title: firstString(product["title"], product["name"]), URL: firstString(product["url"], product["desktop_href"], product["href"]), ImageURL: firstString(product["image_url"], product["image"])})
	}
	return result
}

func firstSlice(values ...any) []any {
	for _, value := range values {
		if result := asSlice(value); len(result) > 0 {
			return result
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
