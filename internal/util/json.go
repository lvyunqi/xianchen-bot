package util

import "encoding/json"

func JSON(value any) string                    { raw, _ := json.Marshal(value); return string(raw) }
func DecodeJSON(data []byte, target any) error { return json.Unmarshal(data, target) }
