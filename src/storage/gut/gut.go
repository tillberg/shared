
package gut

import (
  "crypto/sha256"
  "../../types"
)

type Storage struct {
  Root string
}

func (*Storage) Get(hash types.Hash) ([]byte, error) {
  // cachePath := GetCachePath(hash)
  // _, err := os.Stat(cachePath)
  return nil, nil
}

func (*Storage) Put(bytes []byte)    (types.Hash, error) {
  // Save a copy in the cache if we don't already have one
  // cachePath := GetCachePath(blob.Hash())
  // _, err := os.Stat(cachePath)
  // if err != nil {
  //   os.MkdirAll(path.Dir(cachePath), 0755)
  //   ioutil.WriteFile(cachePath, blob.bytes, 0644)
  //   log.Printf("Cached %s", blob.ShortHashString())
  // }
  return nil, nil
}

var SHA256_SALT_BEFORE = []byte{'s', 'h', 'a', 'r', 'e', 'd', '('}
var SHA256_SALT_AFTER = []byte{')'}

func calculateHash(bytes []byte) types.Hash {
  h := sha256.New()
  h.Write(SHA256_SALT_BEFORE)
  h.Write(bytes)
  h.Write(SHA256_SALT_AFTER)
  return h.Sum([]byte{})
}

// func GetCachePath(root string, hash types.Hash) string {
//   hashString := GetHexString(hash)
//   return path.Join(root, hashString[:2], hashString[2:])
// }

