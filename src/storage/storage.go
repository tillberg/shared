
package storage

import (
  "log"
  conf "github.com/tillberg/goconfig"
  "../types"
  "./gut"
)

type Storage interface {
  Get(hash types.Hash) (types.Blob, error)
  Put(blob types.Blob) (types.Hash, error)
  Deflate(in []byte) []byte
  Inflate(in []byte) ([]byte, error)
  PutRef(name string, hash types.Hash) error
  GetRef(name string) (types.Hash, error)
}

var CacheRoot = ""

func Configured() Storage {
  config, err := conf.ReadConfigFile("shared.ini")
  types.Check(err)
  storage, err := config.GetString("main", "storage")
  types.Check(err)
  if storage == "gut" {
    return Storage(&gut.Storage{RootPath: CacheRoot})
  } else {
    log.Fatalf("Unrecognized storage configured: %s", storage)
  }
  return nil
}
