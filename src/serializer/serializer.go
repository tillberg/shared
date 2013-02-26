
package serializer

import (
  "log"
  "github.com/tillberg/goconfig/conf"
)

type Serializer interface {
  UnmarshalBranch(bytes []byte) (*types.Branch, error)
  UnmarshalFile  (bytes []byte) (*types.File,   error)
  UnmarshalCommit(bytes []byte) (*types.Commit, error)
  UnmarshalTree  (bytes []byte) (*types.Tree,   error)

  MarshalBranch(branch *types.Branch) []byte, error
  MarshalFile  (file   *types.File)   []byte, error
  MarshalCommit(commit *types.Commit) []byte, error
  MarshalTree  (tree   *types.Tree)   []byte, error
}

func GetConfiguredSerializer() *Serializer {
  config, err := conf.ReadConfigFile("shared.ini")
  check(err)
  serializer, err = config.GetString("main", "serializer")
  check(err)
  if serializer == "gut" {
    return *Serializer(&gut.Serializer{})
  } else if serializer == "proto" {
    return *Serializer(&proto.Serializer{})
  } else {
    log.Fatalf("Unrecognized serializer configured: %s", serializer)
  }
}
