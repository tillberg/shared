
package types

import (
  "log"
  "../sharedpb"
)

func Check(err interface{}) {
  if err != nil {
    log.Fatal(err)
  }
}

type BlobRequest struct {
  Hash            Hash
  ResponseChannel chan Hash
}

type BranchSubscription struct {
  Name            string
  ResponseChannel chan BranchStatus
}

type BranchStatus struct {
  Name string
  Hash Hash
}

var BlobRequestChannel     = make(chan BlobRequest, 100)
var BranchSubscribeChannel = make(chan BranchSubscription, 100)
var BranchUpdateChannel    = make(chan BranchStatus, 100)
var BlobReceiveChannel     = make(chan Blob, 100)
var BlobServicerChannel    = make(chan chan *sharedpb.Message, 10)

type Hash []byte

type HashedBlob struct {
  Hash Hash
  Blob Blob
}

type Blob struct {
  // Only one of these should ever be defined:
  File   *File
  Branch *Branch
  Commit *Commit
  Tree   *Tree
}

type File struct {
  Hash  Hash
  Bytes []byte
}

type Branch struct {
  Name   string
  Commit Hash
}

type Commit struct {
  Text    string
  Tree    Hash
  Parents []Hash
}

type Tree struct {
  Entries []*TreeEntry
}

type TreeEntry struct {
  Hash Hash
  Name string
  Flags uint32
}
