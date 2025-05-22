package serializer

import (
	"github.com/bytedance/sonic"
)

var _ Serializer = (*sonicJson)(nil)

type sonicJson struct{}

func NewSonicJson() Serializer {
	return &sonicJson{}
}

// Marshal implements Serializer.
func (s *sonicJson) Marshal(v any) ([]byte, error) {
	return sonic.Marshal(v)
}

// Unmarshal implements Serializer.
func (s *sonicJson) Unmarshal(data []byte, v any) error {
	return sonic.Unmarshal(data, v)
}
