package amazon

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	mcp "github.com/daltoniam/switchboard"
)

var cartItemCountRe = regexp.MustCompile(`\((\d+)\s+item`)

func getCart(ctx context.Context, a *amazon, _ map[string]any) (*mcp.ToolResult, error) {
	doc, err := a.fetch(ctx, a.cartURL())
	if err != nil {
		return mcp.ErrResult(err)
	}

	type cartItem struct {
		Title        string `json:"title"`
		Price        string `json:"price,omitempty"`
		Quantity     int    `json:"quantity"`
		Image        string `json:"image,omitempty"`
		ProductURL   string `json:"product_url,omitempty"`
		ASIN         string `json:"asin,omitempty"`
		Availability string `json:"availability,omitempty"`
		IsSelected   bool   `json:"is_selected"`
	}

	type cart struct {
		IsEmpty    bool       `json:"is_empty"`
		Items      []cartItem `json:"items"`
		Subtotal   string     `json:"subtotal,omitempty"`
		TotalItems int        `json:"total_items"`
	}

	pageText := doc.Text()
	if strings.Contains(pageText, "Your Amazon Cart is empty") || strings.Contains(pageText, "Your Amazon Basket is empty") {
		return mcp.JSONResult(cart{IsEmpty: true, Items: []cartItem{}, TotalItems: 0})
	}

	var items []cartItem
	doc.Find("#sc-active-cart [data-asin]").Each(func(_ int, s *goquery.Selection) {
		asin, _ := s.Attr("data-asin")
		if asin == "" {
			return
		}

		title := strings.TrimSpace(s.Find("a.sc-product-title .a-truncate-full").Text())
		if title == "" {
			title = strings.TrimSpace(s.Find("a.sc-product-title").Text())
		}

		price := strings.TrimSpace(s.Find(".apex-price-to-pay-value .a-offscreen").Text())
		if price == "" {
			price = strings.TrimSpace(s.Find(".sc-product-price").Text())
		}

		qty := 1
		qtyText := strings.TrimSpace(s.Find(`[data-a-selector="value"]`).Text())
		if qtyText == "" {
			qtyText, _ = s.Find("select.sc-quantity-textfield option[selected]").Attr("value")
		}
		if n, e := strconv.Atoi(qtyText); e == nil && n > 0 {
			qty = n
		}

		imgURL, _ := s.Find(".sc-product-image").Attr("src")
		productURL, _ := s.Find(".sc-product-link").Attr("href")
		availability := strings.TrimSpace(s.Find(".sc-product-availability").Text())

		isSelected := false
		s.Find(`input[type="checkbox"]`).Each(func(_ int, cb *goquery.Selection) {
			if _, checked := cb.Attr("checked"); checked {
				isSelected = true
			}
		})

		items = append(items, cartItem{
			Title:        title,
			Price:        price,
			Quantity:     qty,
			Image:        imgURL,
			ProductURL:   productURL,
			ASIN:         asin,
			Availability: availability,
			IsSelected:   isSelected,
		})
	})

	subtotal := strings.TrimSpace(doc.Find("#sc-subtotal-amount-activecart .sc-price").Text())
	totalItems := len(items)
	label := doc.Find("#sc-subtotal-label-activecart").Text()
	if m := cartItemCountRe.FindStringSubmatch(label); len(m) > 1 {
		if n, e := strconv.Atoi(m[1]); e == nil {
			totalItems = n
		}
	}

	return mcp.JSONResult(cart{
		IsEmpty:    len(items) == 0,
		Items:      items,
		Subtotal:   subtotal,
		TotalItems: totalItems,
	})
}

func addToCart(ctx context.Context, a *amazon, args map[string]any) (*mcp.ToolResult, error) {
	asin := argStr(args, "asin")
	if !asinRe.MatchString(asin) {
		return mcp.ErrResult(fmt.Errorf("asin must be exactly 10 uppercase alphanumeric characters (e.g. B0CHXKM5GK)"))
	}

	doc, err := a.fetch(ctx, a.productURL(asin))
	if err != nil {
		return mcp.ErrResult(err)
	}

	form := doc.Find("#addToCart")
	if form.Length() == 0 {
		return mcp.ErrResult(fmt.Errorf("add-to-cart form not found on product page for ASIN %s", asin))
	}

	actionURL, exists := form.Attr("action")
	if !exists || actionURL == "" {
		actionURL = "/gp/product/handle-buy-box/ref=dp_start-bbf_1_glance"
	}

	if !strings.HasPrefix(actionURL, "http") {
		actionURL = fmt.Sprintf("https://www.%s%s", a.domain, actionURL)
	}

	formData := url.Values{}
	form.Find("input[type=hidden]").Each(func(_ int, s *goquery.Selection) {
		name, _ := s.Attr("name")
		val, _ := s.Attr("value")
		if name != "" {
			formData.Set(name, val)
		}
	})
	formData.Set("submit.add-to-cart", "Add to Cart")
	formData.Set("quantity", "1")

	req, err := http.NewRequestWithContext(ctx, "POST", actionURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return mcp.ErrResult(err)
	}
	a.setCookies(req)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", a.productURL(asin))

	resp, err := a.client.Do(req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	bodyStr := string(body)

	success := strings.Contains(bodyStr, "Added to cart") ||
		strings.Contains(bodyStr, "Added to basket") ||
		strings.Contains(bodyStr, "Added to Cart") ||
		strings.Contains(bodyStr, "sw-atc-confirmation") ||
		resp.StatusCode < 400

	type addResult struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	if success {
		return mcp.JSONResult(addResult{Success: true, Message: fmt.Sprintf("Product %s added to cart", asin)})
	}
	return mcp.JSONResult(addResult{Success: false, Message: fmt.Sprintf("Failed to add product %s to cart", asin)})
}

func clearCart(ctx context.Context, a *amazon, _ map[string]any) (*mcp.ToolResult, error) {
	doc, err := a.fetch(ctx, a.cartURL())
	if err != nil {
		return mcp.ErrResult(err)
	}

	items := doc.Find("#sc-active-cart [data-asin]")
	count := items.Length()

	if count == 0 {
		type clearResult struct {
			Success      bool   `json:"success"`
			Message      string `json:"message"`
			ItemsRemoved int    `json:"items_removed"`
		}
		return mcp.JSONResult(clearResult{Success: true, Message: "Cart is already empty", ItemsRemoved: 0})
	}

	removed := 0
	var lastErr error

	items.Each(func(_ int, s *goquery.Selection) {
		asin, _ := s.Attr("data-asin")
		if asin == "" {
			return
		}

		deleteForm := s.Find(`form[action*="delete"]`)
		if deleteForm.Length() == 0 {
			deleteForm = s.Find(`input[name="submit.delete.C"]`).Closest("form")
		}

		if deleteForm.Length() > 0 {
			actionURL, _ := deleteForm.Attr("action")
			if actionURL == "" {
				return
			}
			if !strings.HasPrefix(actionURL, "http") {
				actionURL = fmt.Sprintf("https://www.%s%s", a.domain, actionURL)
			}

			formData := url.Values{}
			deleteForm.Find("input[type=hidden]").Each(func(_ int, inp *goquery.Selection) {
				name, _ := inp.Attr("name")
				val, _ := inp.Attr("value")
				if name != "" {
					formData.Set(name, val)
				}
			})

			req, e := http.NewRequestWithContext(ctx, "POST", actionURL, strings.NewReader(formData.Encode()))
			if e != nil {
				lastErr = e
				return
			}
			a.setCookies(req)
			req.Header.Set("User-Agent", userAgent)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			resp, e := a.client.Do(req)
			if e != nil {
				lastErr = e
				return
			}
			_ = resp.Body.Close()
			removed++
			time.Sleep(800 * time.Millisecond)
		}
	})

	type clearResult struct {
		Success      bool   `json:"success"`
		Message      string `json:"message"`
		ItemsRemoved int    `json:"items_removed"`
	}

	if lastErr != nil && removed == 0 {
		return mcp.ErrResult(lastErr)
	}

	return mcp.JSONResult(clearResult{
		Success:      true,
		Message:      fmt.Sprintf("Removed %d item(s) from cart", removed),
		ItemsRemoved: removed,
	})
}
