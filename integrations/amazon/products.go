package amazon

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	mcp "github.com/daltoniam/switchboard"
)

const maxSearchResults = 20

func searchProducts(ctx context.Context, a *amazon, args map[string]any) (*mcp.ToolResult, error) {
	term := argStr(args, "search_term")
	if term == "" {
		return errResult(fmt.Errorf("search_term is required"))
	}

	doc, err := a.fetch(ctx, a.searchURL(term))
	if err != nil {
		return errResult(err)
	}

	type reviewInfo struct {
		AverageRating string `json:"average_rating,omitempty"`
		ReviewCount   string `json:"review_count,omitempty"`
	}

	type product struct {
		ASIN            string      `json:"asin"`
		Title           string      `json:"title"`
		IsSponsored     bool        `json:"is_sponsored"`
		Brand           string      `json:"brand,omitempty"`
		Price           string      `json:"price,omitempty"`
		PricePerUnit    string      `json:"price_per_unit,omitempty"`
		Reviews         *reviewInfo `json:"reviews,omitempty"`
		ImageURL        string      `json:"image_url,omitempty"`
		IsPrimeEligible bool        `json:"is_prime_eligible"`
		DeliveryInfo    string      `json:"delivery_info,omitempty"`
		ProductURL      string      `json:"product_url,omitempty"`
	}

	var products []product

	doc.Find(`[role="listitem"]`).Each(func(_ int, s *goquery.Selection) {
		if len(products) >= maxSearchResults {
			return
		}

		asin, _ := s.Attr("data-asin")
		if asin == "" || len(asin) != 10 {
			return
		}

		titleSel := s.Find("h2[aria-label]")
		title := strings.TrimSpace(titleSel.Text())
		if title == "" {
			return
		}

		isSponsored := strings.Contains(title, "Sponsored")
		title = strings.TrimPrefix(title, "Sponsored Ad – ")
		title = strings.TrimPrefix(title, "Sponsored Ad - ")

		brand := strings.TrimSpace(s.Find("h2.a-size-mini span.a-size-base-plus.a-color-base").Text())
		price := strings.TrimSpace(s.Find(`span.a-price[data-a-size="xl"] > span.a-offscreen`).Text())
		pricePerUnit := strings.TrimSpace(s.Find(`span.a-price[data-a-size="b"][data-a-color="secondary"] > span.a-offscreen`).Text())

		var rev *reviewInfo
		rating := strings.TrimSpace(s.Find("i.a-icon-star-mini span.a-icon-alt").Text())
		count := strings.TrimSpace(s.Find(`a[aria-label] span.a-size-small`).Text())
		if rating != "" || count != "" {
			rev = &reviewInfo{AverageRating: rating, ReviewCount: count}
		}

		imgURL, _ := s.Find("img.s-image").Attr("src")
		isPrime := s.Find("i.a-icon-prime").Length() > 0
		delivery := strings.TrimSpace(s.Find("div.udm-primary-delivery-message").Text())

		products = append(products, product{
			ASIN:            asin,
			Title:           title,
			IsSponsored:     isSponsored,
			Brand:           brand,
			Price:           price,
			PricePerUnit:    pricePerUnit,
			Reviews:         rev,
			ImageURL:        imgURL,
			IsPrimeEligible: isPrime,
			DeliveryInfo:    delivery,
			ProductURL:      a.productURL(asin),
		})
	})

	return jsonResult(products)
}

func getProduct(ctx context.Context, a *amazon, args map[string]any) (*mcp.ToolResult, error) {
	asin := argStr(args, "asin")
	if asin == "" || len(asin) != 10 {
		return errResult(fmt.Errorf("asin must be exactly 10 characters"))
	}

	doc, err := a.fetch(ctx, a.productURL(asin))
	if err != nil {
		return errResult(err)
	}

	title := strings.TrimSpace(doc.Find("span#productTitle").Text())

	price := strings.TrimSpace(doc.Find("#subscriptionPrice .a-price .a-offscreen").Text())
	if price == "" {
		price = strings.TrimSpace(doc.Find(".priceToPay .a-offscreen").First().Text())
	}
	if price == "" {
		price = strings.TrimSpace(doc.Find(".a-price .a-offscreen").First().Text())
	}

	canSubscribe := doc.Find("#subscriptionPrice").Length() > 0

	overview := cleanText(doc.Find("#productOverview_feature_div").Text())
	features := cleanText(doc.Find("#featurebullets_feature_div").Text())
	facts := cleanText(doc.Find("#productFactsDesktop_feature_div").Text())
	brandSnap := cleanText(doc.Find("#brandSnapshot_feature_div").Text())

	avgRating := strings.TrimSpace(doc.Find("#averageCustomerReviews span.a-size-small.a-color-base").Text())
	reviewCount := ""
	doc.Find("#acrCustomerReviewLink span").Each(func(_ int, s *goquery.Selection) {
		if label, exists := s.Attr("aria-label"); exists {
			reviewCount = label
		} else {
			text := strings.TrimSpace(s.Text())
			if text != "" && reviewCount == "" {
				reviewCount = text
			}
		}
	})

	mainImage, _ := doc.Find("#main-image-container img.a-dynamic-image, #imgTagWrapperId img").Attr("src")

	type description struct {
		Overview      string `json:"overview,omitempty"`
		Features      string `json:"features,omitempty"`
		Facts         string `json:"facts,omitempty"`
		BrandSnapshot string `json:"brand_snapshot,omitempty"`
	}

	type reviews struct {
		AverageRating string `json:"average_rating,omitempty"`
		ReviewsCount  string `json:"reviews_count,omitempty"`
	}

	type productDetail struct {
		ASIN                    string      `json:"asin"`
		Title                   string      `json:"title"`
		Price                   string      `json:"price,omitempty"`
		CanUseSubscribeAndSave  bool        `json:"can_use_subscribe_and_save"`
		Description             description `json:"description"`
		Reviews                 reviews     `json:"reviews"`
		MainImageURL            string      `json:"main_image_url,omitempty"`
	}

	result := productDetail{
		ASIN:                   asin,
		Title:                  title,
		Price:                  price,
		CanUseSubscribeAndSave: canSubscribe,
		Description: description{
			Overview:      overview,
			Features:      features,
			Facts:         facts,
			BrandSnapshot: brandSnap,
		},
		Reviews: reviews{
			AverageRating: avgRating,
			ReviewsCount:  reviewCount,
		},
		MainImageURL: mainImage,
	}

	return jsonResult(result)
}

var multiSpaceRe = regexp.MustCompile(`\s{2,}`)

func cleanText(s string) string {
	s = strings.TrimSpace(s)
	s = multiSpaceRe.ReplaceAllString(s, " ")
	return s
}
