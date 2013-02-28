
package gut

import (
  "encoding/hex"
  "fmt"
  "io/ioutil"
  // "log"
  "os"
  "path"
  "../../serializer"
  "../../types"
)

type Storage struct {
  RootPath string
}

func (s *Storage) getCachePath(hash types.Hash) string {
  hashString := hex.EncodeToString(hash)
  return path.Join(s.RootPath, "objects", hashString[:2], hashString[2:])
}

func (s *Storage) Get(hash types.Hash) (blob types.Blob, err error) {
  cachePath := s.getCachePath(hash)
  _, err = os.Stat(cachePath)
  if err == nil {
    data, err := ioutil.ReadFile(cachePath)
    if err == nil {
      blob, err = serializer.Configured().Unmarshal(data)
    }
  }
  return blob, err
}

func (s *Storage) Put(blob types.Blob) (hash types.Hash, err error) {
  hash, data, err := serializer.Configured().Marshal(blob)
  cachePath := s.getCachePath(hash)
  _, err = os.Stat(cachePath)
  if err != nil {
    os.MkdirAll(path.Dir(cachePath), 0755)
    // log.Printf("Saving %s to cache (%d bytes)", hex.EncodeToString(hash)[:16], len(data))
    err = ioutil.WriteFile(cachePath, data, 0644)
  }
  return hash, err
}

func (s *Storage) PutRef(name string, hash types.Hash) error {
  refPath := path.Join(s.RootPath, "refs", "heads", name)
  os.MkdirAll(path.Dir(refPath), 0755)
  ioutil.WriteFile(refPath, []byte(fmt.Sprintf("%s\n", hex.EncodeToString(hash))), 0644)
  return nil
}

func (s *Storage) GetRef(name string) (types.Hash, error) {
  return nil, nil
}
