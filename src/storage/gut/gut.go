
package gut

import (
  "crypto/sha1"
  "encoding/hex"
  "io/ioutil"
  // "log"
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
  h.Write(bytes)
  return h.Sum([]byte{})
}

func (s *Storage) Get(hash types.Hash) (data []byte, err error) {
  cachePath := s.getCachePath(hash)
  _, err = os.Stat(cachePath)
  if err == nil {
    data, _ = ioutil.ReadFile(cachePath)
  }
  return data, err
}

func (s *Storage) Put(data []byte) (hash types.Hash, err error) {
  hash = calculateHash(data)
  cachePath := s.getCachePath(hash)
  _, err = os.Stat(cachePath)
  if err != nil {
    os.MkdirAll(path.Dir(cachePath), 0755)
    // log.Printf("Saving %s to cache (%d bytes)", hex.EncodeToString(hash)[:16], len(data))
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
