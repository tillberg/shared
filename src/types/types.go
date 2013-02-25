
package types

import "../sharedpb"

type BlobRequest struct {
  Hash            []byte
  ResponseChannel chan []byte
}

type BranchSubscription struct {
  Name          string
  ResponseChannel chan BranchStatus
}

type BranchStatus struct {
  Name string
  Hash []byte
}

var BlobRequestChannel = make(chan BlobRequest, 100)
var BranchSubscribeChannel = make(chan BranchSubscription, 100)
var BranchUpdateChannel = make(chan BranchStatus, 100)
var BlobReceiveChannel = make(chan []byte, 100)
var BlobServicerChannel = make(chan chan *sharedpb.Message, 10)