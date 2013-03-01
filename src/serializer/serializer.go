
package serializer

import (
  "log"
  "github.com/tillberg/goconfig/conf"
  "../types"
  "./gut"
  "./proto"
)

type Serializer interface {
  Unmarshal(bytes []byte)   (types.Blob, error)
  Marshal(blob types.Blob) ([]byte, error)
}

func Configured() Serializer {
  config, err := conf.ReadConfigFile("shared.ini")
  types.Check(err)
  serializer, err := config.GetString("main", "serializer")
  types.Check(err)
  if serializer == "gut" {
    return Serializer(&gut.Serializer{})
  } else if serializer == "proto" {
    return Serializer(&proto.Serializer{})
  } else {
    log.Fatalf("Unrecognized serializer configured: %s", serializer)
  }
  return nil
}
