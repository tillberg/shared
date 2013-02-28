
package proto

import (
  // "../../sharedpb"
  "../../types"
)

type Serializer struct {}

func (s *Serializer) Unmarshal(bytes []byte) (types.Blob, error) {
  return types.Blob{}, nil
}

func (s *Serializer) Marshal(blob types.Blob) (types.Hash, []byte, error) {
  return nil, nil, nil
}
