package serializer

import (
	"encoding/json"
)

var _ Serializer = (*stdJson)(nil)

// 标准库 JSON（还有一个 sonic json）
type stdJson struct{}

func NewStdJson() Serializer {
	return &stdJson{}
}

// Marshal implements Serializer.
func (s *stdJson) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal implements Serializer.
func (s *stdJson) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
