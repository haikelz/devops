package models

type Reaction struct {
	Slug string `json:"slug"`
	Love int64  `json:"love"`
}

func (Reaction) TableName() string {
	return "reactions"
}
