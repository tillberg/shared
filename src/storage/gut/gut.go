
package gut

import (
  "crypto/sha1"
  "encoding/hex"
  "fmt"
  "io/ioutil"
  "os"
  "path"
  "../../types"
)

type Storage struct {
  RootPath string
}

func (s *Storage) getCachePath(hash types.Hash) string {
  hashString := hex.EncodeToString(hash)
  return path.Join(s.RootPath, "objects", hashString[:2], hashString[2:])
}

func calculateHash(bytes []byte) types.Hash {
  h := sha1.New()
  h.Write([]byte(fmt.Sprintf("blob %d", len(bytes))))
  h.Write([]byte{0})
  h.Write(bytes)
  return h.Sum([]byte{})
}

func (s *Storage) Get(hash types.Hash) (data []byte, err error) {
  cachePath := s.getCachePath(hash)
  _, err = os.Stat(cachePath)
  if err != nil {
    data, err = ioutil.ReadFile(cachePath)
  }
  return data, err
}

func (s *Storage) Put(data []byte) (hash types.Hash, err error) {
  hash = calculateHash(data)
  cachePath := s.getCachePath(hash)
  _, err = os.Stat(cachePath)
  if err != nil {
    os.MkdirAll(path.Dir(cachePath), 0755)
    err = ioutil.WriteFile(cachePath, data, 0644)
  }
  return hash, err
}

func (*Storage) PutRef(name string, hash types.Hash) error {
  return nil
}

func (*Storage) GetRef(name string) (types.Hash, error) {
  return nil, nil
}
