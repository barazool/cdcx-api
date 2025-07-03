package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/b-thark/cdcx-api/pkg/types"
)

type Fetcher struct {
	baseURL string
	client  *http.Client
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		baseURL: "https://api.coindcx.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (f *Fetcher) GetMarketDetails() ([]types.MarketDetail, error) {
	url := f.baseURL + "/exchange/v1/markets_details"

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %v", err)
	}

	var markets []types.MarketDetail
	if err := json.Unmarshal(body, &markets); err != nil {
		return nil, fmt.Errorf("parse error: %v", err)
	}

	return markets, nil
}

func (f *Fetcher) GetOrderBook(pair string) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://public.coindcx.com/market_data/orderbook?pair=%s", pair)

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %v", err)
	}

	var orderBook map[string]interface{}
	if err := json.Unmarshal(body, &orderBook); err != nil {
		return nil, fmt.Errorf("parse error: %v", err)
	}

	return orderBook, nil
}

func (f *Fetcher) GetTicker() ([]map[string]interface{}, error) {
	url := f.baseURL + "/exchange/ticker"

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %v", err)
	}

	var tickers []map[string]interface{}
	if err := json.Unmarshal(body, &tickers); err != nil {
		return nil, fmt.Errorf("parse error: %v", err)
	}

	return tickers, nil
}
