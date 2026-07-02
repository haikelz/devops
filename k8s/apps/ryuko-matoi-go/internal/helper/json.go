package helper

import "encoding/json"

func DecodeJSON(payload []byte) (any, error) {
	var parsed any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}
