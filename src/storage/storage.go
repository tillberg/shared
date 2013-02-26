
package storage

import (
  "log"
  "../sharedpb"
)

type Storage interface {
  Get(hash []byte) []byte, error
  Put(bytes []byte) []byte, error
}

func GetConfiguredStorage() *Storage {
  config, err := conf.ReadConfigFile("shared.ini")
  check(err)
  storage, err = config.GetString("main", "storage")
  check(err)
  if storage == "gut" {
    return Serializer(&gut.Storage{})
  } else {
    log.Fatalf("Unrecognized storage configured: %s", storage)
  }
}
