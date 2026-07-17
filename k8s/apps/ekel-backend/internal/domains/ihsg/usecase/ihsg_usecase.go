package usecase

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"app/internal/domains/ihsg"
)

type ihsgUsecase struct {
	client     *http.Client
	cache      []ihsg.MarketDataset
	cacheUntil time.Time
	mu         sync.RWMutex
}

type marketConfig struct {
	Code        string
	Label       string
	Title       string
	Symbol      string
	YahooSymbol string
	StooqSymbol string
}

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Close  []*float64 `json:"close"`
					Volume []*float64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

var marketConfigs = []marketConfig{
	{Code: "ID", Label: "Indonesia", Title: "IDX Composite (IHSG)", Symbol: "IHSG", YahooSymbol: "^JKSE", StooqSymbol: "^jkse"},
	{Code: "JP", Label: "Japan", Title: "Nikkei 225", Symbol: "N225", YahooSymbol: "^N225", StooqSymbol: "^nkx"},
	{Code: "US", Label: "America", Title: "S&P 500", Symbol: "SPX", YahooSymbol: "^GSPC", StooqSymbol: "^spx"},
	{Code: "CN", Label: "China", Title: "SSE Composite", Symbol: "SSEC", YahooSymbol: "000001.SS"},
	{Code: "SA", Label: "Saudi Arabia", Title: "Tadawul All Share", Symbol: "TASI", YahooSymbol: "^TASI.SR"},
}

func NewIhsgUsecase() ihsg.IhsgUsecase {
	return &ihsgUsecase{
		client: &http.Client{Timeout: 12 * time.Second},
	}
}

func (uc *ihsgUsecase) GetMarkets(ctx context.Context) ([]ihsg.MarketDataset, error) {
	uc.mu.RLock()
	if len(uc.cache) > 0 && time.Now().Before(uc.cacheUntil) {
		defer uc.mu.RUnlock()
		return uc.cache, nil
	}
	uc.mu.RUnlock()

	markets := make([]ihsg.MarketDataset, len(marketConfigs))
	var wg sync.WaitGroup

	for i, config := range marketConfigs {
		wg.Add(1)
		go func(index int, cfg marketConfig) {
			defer wg.Done()
			markets[index] = uc.getMarketData(cfg)
		}(i, config)
	}

	wg.Wait()

	if len(markets) == 0 {
		return nil, errors.New("no market data returned")
	}

	uc.mu.Lock()
	uc.cache = markets
	uc.cacheUntil = time.Now().Add(5 * time.Minute)
	uc.mu.Unlock()

	return markets, nil
}

func (uc *ihsgUsecase) getMarketData(config marketConfig) ihsg.MarketDataset {
	fetchedAt := time.Now().UTC().Format(time.RFC3339)
	yahooSource := fmt.Sprintf("Yahoo Finance (%s)", config.Symbol)
	yahooFallback := []ihsg.IHSGPoint{}

	if parsedYahoo, err := uc.getYahooData(config.YahooSymbol); err == nil {
		yahooHasVolume := false
		for _, point := range parsedYahoo {
			if point.Volume > 0 {
				yahooHasVolume = true
				break
			}
		}

		if len(parsedYahoo) > 0 && yahooHasVolume {
			return marketDataset(config, yahooSource, fetchedAt, parsedYahoo, "")
		}

		yahooFallback = parsedYahoo
	}

	if config.StooqSymbol != "" {
		if parsedStooq, err := uc.getStooqData(config.StooqSymbol); err == nil && len(parsedStooq) > 0 {
			return marketDataset(config, fmt.Sprintf("Stooq (%s)", config.Symbol), fetchedAt, parsedStooq, "")
		}
	}

	if len(yahooFallback) > 0 {
		return marketDataset(config, yahooSource+" (volume unavailable)", fetchedAt, yahooFallback, "")
	}

	return marketDataset(config, yahooSource, fetchedAt, []ihsg.IHSGPoint{}, fmt.Sprintf("Data %s belum tersedia dari penyedia data.", config.Label))
}

func marketDataset(config marketConfig, source string, fetchedAt string, data []ihsg.IHSGPoint, errorMessage string) ihsg.MarketDataset {
	points := make([]*ihsg.IHSGPoint, len(data))
	for i := range data {
		points[i] = &data[i]
	}
	return ihsg.MarketDataset{
		Code:         config.Code,
		Label:        config.Label,
		Title:        config.Title,
		Symbol:       config.Symbol,
		Source:       source,
		FetchedAt:    fetchedAt,
		Data:         points,
		ErrorMessage: errorMessage,
	}
}

func (uc *ihsgUsecase) getYahooData(symbol string) ([]ihsg.IHSGPoint, error) {
	baseURL := strings.TrimRight(os.Getenv("IHSG_API_URL"), "/")
	if baseURL == "" {
		baseURL = "https://query1.finance.yahoo.com"
	}

	endpoint := fmt.Sprintf("%s/v8/finance/chart/%s?interval=1d&range=5y", baseURL, url.PathEscape(symbol))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	res, err := uc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("yahoo returned status %d", res.StatusCode)
	}

	var payload yahooChartResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Chart.Error != nil {
		return nil, errors.New(payload.Chart.Error.Description)
	}
	if len(payload.Chart.Result) == 0 || len(payload.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, errors.New("empty yahoo chart data")
	}

	result := payload.Chart.Result[0]
	quote := result.Indicators.Quote[0]
	points := make([]ihsg.IHSGPoint, 0, len(result.Timestamp))

	for i, timestamp := range result.Timestamp {
		if i >= len(quote.Open) || i >= len(quote.High) || i >= len(quote.Low) || i >= len(quote.Close) {
			continue
		}
		if quote.Open[i] == nil || quote.High[i] == nil || quote.Low[i] == nil || quote.Close[i] == nil {
			continue
		}

		volume := 0.0
		if i < len(quote.Volume) && quote.Volume[i] != nil {
			volume = *quote.Volume[i]
		}

		point := ihsg.IHSGPoint{
			Date:   time.Unix(timestamp, 0).UTC().Format(time.DateOnly),
			Open:   *quote.Open[i],
			High:   *quote.High[i],
			Low:    *quote.Low[i],
			Close:  *quote.Close[i],
			Volume: volume,
		}

		if point.Date != "" && point.Open > 0 && point.High > 0 && point.Low > 0 && point.Close > 0 {
			points = append(points, point)
		}
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].Date < points[j].Date
	})

	return points, nil
}

func (uc *ihsgUsecase) getStooqData(symbol string) ([]ihsg.IHSGPoint, error) {
	endpoint := "https://stooq.com/q/d/l/?s=" + url.QueryEscape(symbol) + "&i=d"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	res, err := uc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("stooq returned status %d", res.StatusCode)
	}

	return parseStooqCSV(res.Body)
}

func parseStooqCSV(reader io.Reader) ([]ihsg.IHSGPoint, error) {
	rows, err := csv.NewReader(reader).ReadAll()
	if err != nil {
		return nil, err
	}

	points := make([]ihsg.IHSGPoint, 0, len(rows))
	for _, row := range rows[1:] {
		if len(row) < 6 || row[0] == "" || row[1] == "" || row[2] == "" || row[3] == "" || row[4] == "" {
			continue
		}

		open, openErr := strconv.ParseFloat(row[1], 64)
		high, highErr := strconv.ParseFloat(row[2], 64)
		low, lowErr := strconv.ParseFloat(row[3], 64)
		closeValue, closeErr := strconv.ParseFloat(row[4], 64)
		volume := 0.0
		if row[5] != "" {
			volume, _ = strconv.ParseFloat(row[5], 64)
		}

		if openErr != nil || highErr != nil || lowErr != nil || closeErr != nil {
			continue
		}

		point := ihsg.IHSGPoint{
			Date:   row[0],
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closeValue,
			Volume: volume,
		}

		if point.Date != "" && point.Open > 0 && point.High > 0 && point.Low > 0 && point.Close > 0 {
			points = append(points, point)
		}
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].Date < points[j].Date
	})

	return points, nil
}
