
package gut

import (
  "../../types"
)

type Serializer struct {}

func (s *Serializer) Unmarshal(bytes []byte) (*types.Blob, error) {
  return nil, nil
}

func (s *Serializer) Marshal(blob *types.Blob) ([]byte, error) {
  return nil, nil
}
