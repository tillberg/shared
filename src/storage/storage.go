
package storage

import (
  "log"
  "github.com/tillberg/goconfig/conf"
  "../types"
  "./gut"
)

type Storage interface {
  Get(hash types.Hash) ([]byte, error)
  Put(bytes []byte)    (types.Hash, error)
}

var CacheRoot = ""

func Configured() Storage {
  config, err := conf.ReadConfigFile("shared.ini")
  types.Check(err)
  storage, err := config.GetString("main", "storage")
  types.Check(err)
  if storage == "gut" {
    return Storage(&gut.Storage{Root: CacheRoot})
  } else {
    log.Fatalf("Unrecognized storage configured: %s", storage)
  }
  return nil
}