
package types

import "../sharedpb"

type BlobRequest struct {
  Hash            []byte
  ResponseChannel chan []byte
}

type BranchSubscription struct {
  Name            string
  ResponseChannel chan BranchStatus
}

type BranchStatus struct {
  Name string
  Hash []byte
}

var BlobRequestChannel     = make(chan BlobRequest, 100)
var BranchSubscribeChannel = make(chan BranchSubscription, 100)
var BranchUpdateChannel    = make(chan BranchStatus, 100)
var BlobReceiveChannel     = make(chan []byte, 100)
var BlobServicerChannel    = make(chan chan *sharedpb.Message, 10)

type Blob struct {
  // Only one of these should ever be defined:
  File   *File
  Branch *Branch
  Commit *Commit
  Tree   *Tree
}

type File struct {
  Bytes []byte
}

type Branch struct {
  Name   string
  Commit []byte
}

type Commit struct {
  Text    string
  Tree    []byte
  Parents [][]byte
}

type Tree struct {
  Entries []TreeEntry
}

type TreeEntry struct {
  Blob []byte
  Name string
  Flags uint32
}
