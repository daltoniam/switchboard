package amazon

import (
	"context"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	mcp "github.com/daltoniam/switchboard"
)

var (
	collectionDateRe = regexp.MustCompile(`Collected on (.+)`)
	returnDateRe     = regexp.MustCompile(`until (.+)`)
	asinFromURLRe    = regexp.MustCompile(`/dp/([A-Z0-9]{10})`)
)

func getOrders(ctx context.Context, a *amazon, _ map[string]any) (*mcp.ToolResult, error) {
	doc, err := a.fetch(ctx, a.ordersURL())
	if err != nil {
		return errResult(err)
	}

	type deliveryAddress struct {
		Name    string `json:"name,omitempty"`
		Address string `json:"address,omitempty"`
		Country string `json:"country,omitempty"`
	}

	type orderInfo struct {
		OrderNumber    string           `json:"order_number,omitempty"`
		OrderDate      string           `json:"order_date,omitempty"`
		Total          string           `json:"total,omitempty"`
		DeliveryAddr   *deliveryAddress `json:"delivery_address,omitempty"`
		Status         string           `json:"status,omitempty"`
		CollectionDate string           `json:"collection_date,omitempty"`
	}

	type orderItem struct {
		Title          string `json:"title"`
		Image          string `json:"image,omitempty"`
		ProductURL     string `json:"product_url,omitempty"`
		ASIN           string `json:"asin,omitempty"`
		ReturnEligible bool   `json:"return_eligible"`
		ReturnDate     string `json:"return_date,omitempty"`
	}

	type order struct {
		OrderInfo orderInfo   `json:"order_info"`
		Items     []orderItem `json:"items"`
	}

	var orders []order

	doc.Find(".order-card").Each(func(_ int, card *goquery.Selection) {
		orderNum := strings.TrimSpace(card.Find(".yohtmlc-order-id span").Last().Text())
		orderDate := strings.TrimSpace(card.Find(".order-header__header-list-item").First().Find(".a-size-base").Text())
		total := ""
		card.Find(".order-header__header-list-item").Each(func(i int, s *goquery.Selection) {
			if i == 1 {
				total = strings.TrimSpace(s.Find(".a-size-base").Text())
			}
		})

		status := strings.TrimSpace(card.Find(".delivery-box__primary-text").Text())
		collectionDate := ""
		if m := collectionDateRe.FindStringSubmatch(status); len(m) > 1 {
			collectionDate = m[1]
		}

		var addr *deliveryAddress
		popover := card.Find(".a-popover-preload")
		if popover.Length() > 0 {
			name := strings.TrimSpace(popover.Find("h5").Text())
			rows := popover.Find(".a-row")
			addrText := ""
			country := ""
			if rows.Length() > 1 {
				addrText = cleanText(rows.Eq(1).Text())
			}
			if rows.Length() > 0 {
				country = strings.TrimSpace(rows.Last().Text())
			}
			if name != "" || addrText != "" || country != "" {
				addr = &deliveryAddress{Name: name, Address: addrText, Country: country}
			}
		}

		info := orderInfo{
			OrderNumber:    orderNum,
			OrderDate:      orderDate,
			Total:          total,
			DeliveryAddr:   addr,
			Status:         status,
			CollectionDate: collectionDate,
		}

		var items []orderItem
		card.Find(".item-box").Each(func(_ int, item *goquery.Selection) {
			titleLink := item.Find("a.yohtmlc-product-title")
			if titleLink.Length() == 0 {
				titleLink = item.Find(".yohtmlc-product-title a")
			}
			title := strings.TrimSpace(titleLink.Text())
			href, _ := titleLink.Attr("href")
			productURL := ""
			asin := ""
			if href != "" {
				productURL = href
				if m := asinFromURLRe.FindStringSubmatch(href); len(m) > 1 {
					asin = m[1]
				}
			}

			imgURL, _ := item.Find(".product-image img").Attr("src")

			returnEligible := false
			returnDate := ""
			item.Find(".a-size-small").Each(func(_ int, s *goquery.Selection) {
				text := strings.TrimSpace(s.Text())
				if strings.Contains(text, "Return") || strings.Contains(text, "Replace") {
					returnEligible = true
					if m := returnDateRe.FindStringSubmatch(text); len(m) > 1 {
						returnDate = m[1]
					}
				}
			})

			items = append(items, orderItem{
				Title:          title,
				Image:          imgURL,
				ProductURL:     productURL,
				ASIN:           asin,
				ReturnEligible: returnEligible,
				ReturnDate:     returnDate,
			})
		})

		orders = append(orders, order{OrderInfo: info, Items: items})
	})

	return jsonResult(orders)
}
