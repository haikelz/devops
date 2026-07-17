package wakatime

type StatsResponse struct {
	TotalSeconds                       float64        `json:"total_seconds"`
	HumanReadableTotal                 string         `json:"human_readable_total"`
	DailyAverage                       float64        `json:"daily_average"`
	HumanReadableDailyAverage          string         `json:"human_readable_daily_average"`
	Languages                          []LangStat     `json:"languages"`
	Projects                           []ProjectStat  `json:"projects"`
	OperatingSystems                   []OSStat       `json:"operating_systems"`
	Editors                            []EditorStat   `json:"editors"`
	Categories                         []CategoryStat `json:"categories"`
	Range                              string         `json:"range"`
	HumanReadableRange                 string         `json:"human_readable_range"`
	IsIncludingToday                   bool           `json:"is_including_today"`
}

type LangStat struct {
	Name         string  `json:"name"`
	TotalSeconds float64 `json:"total_seconds"`
	Percent      float64 `json:"percent"`
	Digital      string  `json:"digital"`
	Text         string  `json:"text"`
	Hours        int     `json:"hours"`
	Minutes      int     `json:"minutes"`
}

type ProjectStat struct {
	Name         string  `json:"name"`
	TotalSeconds float64 `json:"total_seconds"`
	Percent      float64 `json:"percent"`
	Digital      string  `json:"digital"`
	Text         string  `json:"text"`
	Hours        int     `json:"hours"`
	Minutes      int     `json:"minutes"`
}

type OSStat struct {
	Name         string  `json:"name"`
	TotalSeconds float64 `json:"total_seconds"`
	Percent      float64 `json:"percent"`
	Digital      string  `json:"digital"`
	Text         string  `json:"text"`
	Hours        int     `json:"hours"`
	Minutes      int     `json:"minutes"`
}

type EditorStat struct {
	Name         string  `json:"name"`
	TotalSeconds float64 `json:"total_seconds"`
	Percent      float64 `json:"percent"`
	Digital      string  `json:"digital"`
	Text         string  `json:"text"`
	Hours        int     `json:"hours"`
	Minutes      int     `json:"minutes"`
}

type CategoryStat struct {
	Name         string  `json:"name"`
	TotalSeconds float64 `json:"total_seconds"`
	Percent      float64 `json:"percent"`
	Digital      string  `json:"digital"`
	Text         string  `json:"text"`
	Hours        int     `json:"hours"`
	Minutes      int     `json:"minutes"`
}
