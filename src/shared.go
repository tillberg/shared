package main

import (
  "bytes"
  "flag"
  "os"
  "os/signal"
  "log"
  "./blob"
  "./sharedpb"
  "./network"
  "./storage"
  "./types"
  "github.com/howeyc/fsnotify"
  "github.com/tillberg/goconfig/conf"
)

func check(err interface{}) {
  if err != nil {
    log.Fatal(err)
  }
}

var watch_target *string = flag.String("watch", "_sync", "The directory to sync")
var cache_root *string = flag.String("cache", "_cache", "Directory to keep cache of objects")
var listen_port *int = flag.Int("port", 9251, "Port to listen on")

//
// var branchStatusChannel = make(chan *sharedpb.Message, 10)
// var subscribeChannel = make(chan chan *sharedpb.Message, 10)
// var objectReceiveChannel = make(chan *Blob, 100)
// var branchReceiveChannel = make(chan []byte, 10)
// var BranchSubscribeChannel = make(chan *BranchSubscription, 10)

func ArbitBlobRequests() {
  servicers := []chan *sharedpb.Message{}
  subscribers := map[string][]chan types.Hash{}
  for {
    select {
      case servicer := <-types.BlobServicerChannel:
        servicers = append(servicers, servicer)
      case request := <-types.BlobRequestChannel:
        for _, servicer := range servicers {
          servicer <- &sharedpb.Message{HashRequest: request.Hash}
        }
        hashString := blob.GetHexString(request.Hash)
        // log.Printf("Waiting for %s", blob.GetShortHexString(request.Hash))
        if subscribers[hashString] == nil {
          subscribers[hashString] = []chan types.Hash{}
        }
        subscribers[hashString] = append(subscribers[hashString], request.ResponseChannel)
      case receivedBlob := <-types.BlobReceiveChannel:
        hash, err := storage.Configured().Put(receivedBlob)
        check(err)
        // log.Printf("Forwarding %s", blob.GetShortHexString(hash))
        for _, subscriber := range subscribers[blob.GetHexString(hash)] {
          subscriber <- hash
        }
    }
  }
}

func ArbitBranchStatus() {
  subscribers := map[string][]chan types.BranchStatus{}
  statuses := map[string]*types.BranchStatus{}
  for {
    select {
      case subscription := <-types.BranchSubscribeChannel:
        branch := subscription.Name
        if subscribers[branch] == nil {
          subscribers[branch] = []chan types.BranchStatus{}
        }
        subscribers[branch] = append(subscribers[branch], subscription.ResponseChannel)
        if statuses[branch] != nil {
          subscription.ResponseChannel <- *statuses[branch]
        }
      case branchStatus := <-types.BranchUpdateChannel:
        branch := branchStatus.Name
        isNew := statuses[branch] == nil
        if !isNew && !bytes.Equal(branchStatus.Hash, statuses[branch].Hash) {
          query := types.BranchAncestryQuery{
            CommitA: branchStatus.Hash,
            CommitB: statuses[branch].Hash,
            ResponseChannel: make(chan bool),
          }
          types.DoesADescendFromBChannel <- query
          isNew = <-query.ResponseChannel
        }
        if isNew {
          log.Printf("Updating %s -> %s", branch, blob.GetShortHexString(branchStatus.Hash))
          statuses[branch] = &branchStatus
          for _, subscriber := range subscribers[branch] {
            subscriber <- branchStatus
          }
        } else {
          log.Printf("Ignoring %s -> %s", branch, blob.GetShortHexString(branchStatus.Hash))
        }
    }
  }
}

func ArbitCommitHierarchy() {
  commits := map[string]types.Commit{}
  DoesADescendFromB := func(a types.Hash, b types.Hash) bool {
    stack := []types.Hash{a}
    for len(stack) > 0 {
      next := stack[0]
      if bytes.Equal(next, b) {
        return true
      }
      stack = stack[1:]
      str := blob.GetHexString(next)
      commit, present := commits[str]
      if !present {
        commitBlob := blob.GetBlob(next)
        if commitBlob.Commit == nil {
          log.Printf("%#v", commitBlob)
          log.Fatalf("Could not find commit %s", blob.GetShortHexString(next))
        }
        commit = *commitBlob.Commit
        commits[str] = commit
      }
      for _, parent := range commit.Parents {
        stack = append(stack, parent)
      }
    }
    return false
  }
  for {
    select {
      case query := <- types.DoesADescendFromBChannel:
        descends := DoesADescendFromB(query.CommitA, query.CommitB)
        desc := "descends"
        if !descends {
          desc = "does not descend"
        }
        log.Printf("%s %s from %s", blob.GetShortHexString(query.CommitA),
                   desc, blob.GetShortHexString(query.CommitB))
        query.ResponseChannel <- descends
    }
  }
}

func restartOnChange() {
  watcher, _ := fsnotify.NewWatcher()
  watcher.Watch("shared.go")
  <-watcher.Event
  os.Exit(0)
}

func main() {
  flag.Parse()
  log.SetFlags(log.Ltime | log.Lshortfile)
  storage.CacheRoot = *cache_root
  _, err := conf.ReadConfigFile("shared.ini")
  check(err)

  go restartOnChange()

  blob.StartProcessors()

  go ArbitBranchStatus()
  go ArbitBlobRequests()
  go ArbitCommitHierarchy()

  blob.MakeBranch(*watch_target, nil, nil)

  go network.Start(*listen_port)
  interrupt := make(chan os.Signal, 2)
  signal.Notify(interrupt, os.Interrupt)
  <-interrupt
}
