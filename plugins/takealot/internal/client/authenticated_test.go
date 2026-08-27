package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/t12e/takealot-cli/internal/auth"
)

func TestWishlistRoutesAndProductResolution(t *testing.T) {
	backend := &testKeyring{secret: `{"jwt":"jwt","refresh_token":"refresh","customer_id":"42"}`}
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seen = append(seen, request.Method+" "+request.URL.RequestURI())
		writer.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(request.URL.Path, "/customers/") && request.Header.Get("Authorization") != "Bearer jwt" {
			t.Errorf("wishlist request did not include bearer token: %q", request.Header.Get("Authorization"))
		}
		if strings.HasPrefix(request.URL.Path, "/customers/") && !strings.Contains(request.Header.Get("Cookie"), "tal_jwt=jwt") {
			t.Errorf("wishlist request did not include mobile auth cookie: %q", request.Header.Get("Cookie"))
		}
		switch request.URL.Path {
		case "/product-details/PLID123":
			_, _ = writer.Write([]byte(`{"title":"Wheel","desktop_href":"/wheel/PLID123","buybox":{"product_id":456,"tsin":789,"pretty_price":"R 1 000"}}`))
		case "/customers/42/wishlists":
			if request.Method == http.MethodGet {
				_, _ = writer.Write([]byte(`{"page_info":{"total":1},"wishlists":[{"group_id":7,"name":"Sim rig","item_count":1,"private_customer_id":"do-not-leak"}]}`))
			} else {
				_, _ = writer.Write([]byte(`{"group_id":8,"name":"New"}`))
			}
		case "/customers/42/wishlists/items/pid/456":
			if request.Method == http.MethodPut {
				var body map[string]any
				_ = json.NewDecoder(request.Body).Decode(&body)
				if body["reset"] != false || len(body["groups"].([]any)) != 1 {
					t.Errorf("unexpected add body: %#v", body)
				}
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	store := auth.NewStoreWithBackend(backend)
	api := NewAuthenticatedWithHTTPClient(server.Client(), server.URL, server.URL, store)
	result, err := api.ListWishlists(context.Background(), 0, 0)
	if err != nil || len(result.Wishlists) != 1 || result.Wishlists[0].GroupID != "7" {
		t.Fatalf("wishlist list normalization failed: %#v, %v", result, err)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "do-not-leak") || strings.Contains(string(encoded), "private_customer_id") {
		t.Fatalf("wishlist PII leaked: %s", encoded)
	}
	action, err := api.AddProductToWishlist(context.Background(), "7", "https://www.takealot.com/wheel/PLID123")
	if err != nil || !action.Completed || action.Product.ProductID != 456 {
		t.Fatalf("wishlist add failed: %#v, %v", action, err)
	}
	if !containsString(seen, "PUT /customers/42/wishlists/items/pid/456") || !containsString(seen, "GET /product-details/PLID123") {
		t.Fatalf("expected route sequence not observed: %v", seen)
	}
}

type testKeyring struct{ secret string }

func (k *testKeyring) Get(_, _ string) (string, error) { return k.secret, nil }
func (k *testKeyring) Set(_, _, secret string) error   { k.secret = secret; return nil }
func (k *testKeyring) Delete(_, _ string) error        { k.secret = ""; return nil }

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, expected) {
			return true
		}
	}
	return false
}
