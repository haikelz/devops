package ihsg

type IHSGPoint struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

type MarketDataset struct {
	Code         string       `json:"code"`
	Label        string       `json:"label"`
	Title        string       `json:"title"`
	Symbol       string       `json:"symbol"`
	Source       string       `json:"source"`
	FetchedAt    string       `json:"fetchedAt"`
	Data         []*IHSGPoint `json:"data"`
	ErrorMessage string       `json:"errorMessage,omitempty"`
}
