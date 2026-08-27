package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePLID(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "digits", value: "66383997", want: "66383997"},
		{name: "prefixed", value: "PLID66383997", want: "66383997"},
		{name: "url", value: "https://www.takealot.com/volkano-scorpio/PLID66383997", want: "66383997"},
		{name: "product id is ambiguous", value: "product_id=90401948", wantErr: true},
		{name: "tsin is ambiguous", value: "tsin=69563678", wantErr: true},
		{name: "wrong host", value: "https://example.com/product/PLID66383997", wantErr: true},
		{name: "missing plid", value: "https://www.takealot.com/product/66383997", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParsePLID(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("ParsePLID() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestSearchNormalization(t *testing.T) {
	server := fixtureServer(t, map[string]string{
		"/searches/products,filters,facets,sort_options,breadcrumbs,slots_audience,context,seo,layout": `{
  "sections": {"products": {"results": [{"product_views": {
			"core": {"id": 66383997, "title": "Volkano Scorpio", "subtitle": "Wireless Earphones", "brand": "Volkano", "desktop_href": "/product/volkano-scorpio/PLID66383997"},
    "gallery": {"images": [{"url": "https://media.takealot.com/images/earbuds.jpg"}]},
    "buybox_summary": {"product_id": 90401948, "tsin": 69563678, "pretty_price": "R 299", "prices": [299]},
    "review_summary": {"star_rating": 4.7, "review_count": 3044, "distribution": {"1": 47, "2": 7, "3": 2, "4": 686, "5": 2302}},
    "stock_availability_summary": {"status": "In stock", "is_in_stock": true}
  }}]}}
}`,
	})
	result, err := NewWithHTTPClient(server.Client(), server.URL, server.URL).Search(context.Background(), "wireless earbuds", 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.Returned != 1 || result.Results[0].PLID != "66383997" {
		t.Fatalf("unexpected search result: %#v", result)
	}
	product := result.Results[0]
	if product.ProductID != 90401948 || product.TSIN != 69563678 || product.Rating.Count != 3044 || product.Rating.Distribution.FiveStar != 2302 {
		t.Fatalf("identifiers/rating not normalized: %#v", product)
	}
	if product.URL != "https://www.takealot.com/volkano-scorpio/PLID66383997" || len(product.ImageURLs) != 1 {
		t.Fatalf("URL/gallery not normalized: %#v", product)
	}
}

func TestCanonicalProductURL(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "relative legacy product path", value: "/product/moza-sr-p/PLID93563242", want: "https://www.takealot.com/moza-sr-p/PLID93563242"},
		{name: "absolute legacy product path", value: "https://www.takealot.com/product/moza-sr-p/PLID93563242?colour=Black", want: "https://www.takealot.com/moza-sr-p/PLID93563242?colour=Black"},
		{name: "canonical path", value: "https://www.takealot.com/moza-sr-p/PLID93563242", want: "https://www.takealot.com/moza-sr-p/PLID93563242"},
		{name: "other host unchanged", value: "https://example.com/product/moza/PLID93563242", want: "https://example.com/product/moza/PLID93563242"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := productURLWithSlug(test.value, "93563242", ""); got != test.want {
				t.Fatalf("productURLWithSlug() = %q; want %q", got, test.want)
			}
		})
	}
}

func TestProductDetailNormalization(t *testing.T) {
	server := fixtureServer(t, map[string]string{
		"/product-details/PLID123": `{
  "title": "Example Product", "desktop_href": "/example/PLID123",
  "core": {"brand": "Example", "star_rating": 4.5},
  "buybox": {"product_id": 456, "tsin": 789, "pretty_price": "R 499", "prices": [499], "is_add_to_cart_available": true, "seller_detail": {"display_name": "Example Seller", "seller_id": 11}},
  "gallery": {"images": [{"url": "https://media.takealot.com/a.jpg"}, {"src": "https://media.takealot.com/b.jpg"}]},
  "description": {"text": "A useful product.", "html": "<p>A useful product.</p>"},
  "product_information": {"items": [{"display_name": "Warranty", "displayable_text": "2 years", "value": "2 years", "item_type": "text"}]},
  "bullet_point_attributes": {"items": [{"description": "Rechargeable", "positive": true, "type": "feature"}]},
  "reviews": {"count": 12, "star_rating": 4.5, "distribution": {"1": 1, "2": 0, "3": 1, "4": 2, "5": 8}},
  "variants": {"selectors": [{"selector_type": "colour_variant", "title": "Colour", "options": [{"is_enabled": true, "is_selected": true, "href": "/example/PLID123?colour_variant=Black", "value": {"name": "Black", "value": "Black"}}]}]},
  "exchanges_and_returns": {"copy": "https://api.takealot.com/rest/v-1-16-0/product-details/messages/MD_EXCHANGES_AND_RETURNS"}
}`,
	})
	result, err := NewWithHTTPClient(server.Client(), server.URL, server.URL).GetProduct(context.Background(), "https://www.takealot.com/example/PLID123")
	if err != nil {
		t.Fatal(err)
	}
	if result.ProductID != 456 || result.TSIN != 789 || result.Description != "A useful product." || len(result.Gallery) != 2 || len(result.Attributes) != 1 || len(result.Variants) != 1 || result.Returns == nil || result.Returns.URL == "" || result.Stock.InStock == nil || !*result.Stock.InStock {
		t.Fatalf("detail fields not normalized: %#v", result)
	}
	if result.Rating.Distribution.FiveStar != 8 || result.Seller == nil || result.Seller.ID != 11 {
		t.Fatalf("detail rating/seller not normalized: %#v", result)
	}
}

func TestProductImagesDownload(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/product-details/PLID123":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{
  "title": "Example Product",
  "gallery": {"images": [{"url": "` + server.URL + `/images/one.jpg"}, {"url": "` + server.URL + `/images/two.png"}]}
}`))
		case "/images/one.jpg":
			writer.Header().Set("Content-Type", "image/jpeg; charset=binary")
			_, _ = writer.Write([]byte("jpeg image bytes"))
		case "/images/two.png":
			writer.Header().Set("Content-Type", "image/png")
			_, _ = writer.Write([]byte("png image bytes"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	directory := t.TempDir()
	result, err := NewWithHTTPClient(server.Client(), server.URL, server.URL).DownloadProductImages(context.Background(), "123", ImageDownloadOptions{Limit: 2, Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	if result.PLID != "123" || result.Title != "Example Product" || result.Directory != directory || len(result.Images) != 2 {
		t.Fatalf("unexpected image result: %#v", result)
	}
	if result.Images[0].LocalPath != filepath.Join(directory, "01.jpg") || result.Images[1].LocalPath != filepath.Join(directory, "02.png") {
		t.Fatalf("unexpected image paths: %#v", result.Images)
	}
	first, err := os.ReadFile(result.Images[0].LocalPath)
	if err != nil || string(first) != "jpeg image bytes" {
		t.Fatalf("first image was not stored correctly: %q, %v", first, err)
	}
	second, err := os.ReadFile(result.Images[1].LocalPath)
	if err != nil || string(second) != "png image bytes" {
		t.Fatalf("second image was not stored correctly: %q, %v", second, err)
	}
}

func TestProductImagesRejectNonImageResponse(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/product-details/PLID123" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"gallery":{"images":[{"url":"` + server.URL + `/images/not-an-image.jpg"}]}}`))
			return
		}
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte("not an image"))
	}))
	defer server.Close()

	_, err := NewWithHTTPClient(server.Client(), server.URL, server.URL).DownloadProductImages(context.Background(), "123", ImageDownloadOptions{Directory: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "not an image") {
		t.Fatalf("expected non-image error, got %v", err)
	}
}

func TestReviewFiltersPaginationAndPrivacy(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotQuery = request.URL.RawQuery
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
  "page_info": {"total": 2, "total_pages": 1, "current_page": 1, "page_size": 10},
  "sort_options": [{"value": "SO_LATEST", "label": "Latest", "selected": true}],
  "filters": [{"type": "rating", "title": "Rating", "options": [{"value": "5"}]}],
  "reviews": [{"rating": 5, "text": {"body": "Excellent"}, "date": "2026-08-27", "num_upvotes": 3, "customer_name": "Private Name", "customer_id": 42, "signature": "secret", "variant_info": {"colour": "Black"}}]
}`))
	}))
	defer server.Close()
	result, err := NewWithHTTPClient(server.Client(), server.URL, server.URL).GetReviews(context.Background(), "123", ReviewOptions{Rating: 5, Sort: "latest", Page: 1, Variant: "Black"})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"rating=5", "sort=SO_LATEST", "page=1", "colour_variant=Black"} {
		if !strings.Contains(gotQuery, expected) {
			t.Fatalf("query %q does not contain %q", gotQuery, expected)
		}
	}
	if result.Page.Total != 2 || result.Page.CurrentPage != 1 || len(result.Reviews) != 1 || result.Reviews[0].Text != "Excellent" {
		t.Fatalf("review fields not normalized: %#v", result)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	output := string(encoded)
	for _, privateValue := range []string{"Private Name", "customer_id", "signature", "secret"} {
		if strings.Contains(output, privateValue) {
			t.Fatalf("private review value leaked into normalized JSON: %q", privateValue)
		}
	}
}

func TestEmptyReviewsAndHTTPFailures(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantCode   string
		wantSubstr string
	}{
		{name: "empty", status: http.StatusOK, body: `{"page_info":{"total":0,"total_pages":0,"current_page":0,"page_size":10},"reviews":[]}`},
		{name: "forbidden", status: http.StatusForbidden, body: "forbidden", wantCode: "forbidden"},
		{name: "not found", status: http.StatusNotFound, body: "missing", wantCode: "not_found"},
		{name: "rate limited", status: http.StatusTooManyRequests, body: "slow down", wantCode: "rate_limited"},
		{name: "cloudflare", status: http.StatusForbidden, body: "<title>Just a moment...</title>", wantCode: "cloudflare_challenge"},
		{name: "cloudflare body on success status", status: http.StatusOK, body: "<title>Just a moment...</title>", wantCode: "cloudflare_challenge"},
		{name: "malformed", status: http.StatusOK, body: "{not-json", wantSubstr: "malformed JSON"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			result, err := NewWithHTTPClient(server.Client(), server.URL, server.URL).GetReviews(context.Background(), "123", ReviewOptions{})
			if test.name == "empty" {
				if err != nil || result.Page.Total != 0 || len(result.Reviews) != 0 {
					t.Fatalf("unexpected empty result: %#v, %v", result, err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if test.wantCode != "" {
				apiErr, ok := err.(*APIError)
				if !ok || apiErr.Code != test.wantCode {
					t.Fatalf("error = %#v; want API error code %q", err, test.wantCode)
				}
			}
			if test.wantSubstr != "" && !strings.Contains(err.Error(), test.wantSubstr) {
				t.Fatalf("error %q does not contain %q", err, test.wantSubstr)
			}
		})
	}
}

func fixtureServer(t *testing.T, responses map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, ok := responses[request.URL.Path]
		if !ok {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(body))
	}))
}
