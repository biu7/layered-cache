package serializer

type Serializer interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}
