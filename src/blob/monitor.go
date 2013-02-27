
package blob

import (
  "bytes"
  "io/ioutil"
  "log"
  "os"
  "path"
  "time"
  "github.com/howeyc/fsnotify"
  "../serializer"
  "../storage"
  "../types"
)

var processChannel = make(chan FileEvent, 100) // debouncing

func MonitorTree(rootPath string, input chan FileUpdate, mergeChannel chan types.Hash, revisionChannel chan types.Hash) {
  // XXX ideally, this would be a B-Tree with distributed caching
  var children = map[string]*types.TreeEntry{}
  updateSelf := func() {
    tree := &types.Tree{Entries: []*types.TreeEntry{}}
    for name, treeEntry := range children {
      tree.Entries = append(tree.Entries, &types.TreeEntry{
        Hash: treeEntry.Hash,
        Flags: uint32(0644),
        Name: name,
      })
    }
    bytes, err := serializer.Configured().Marshal(&types.Blob{Tree: tree})
    check(err)
    hash, err := storage.Configured().Put(bytes)
    check(err)
    revisionChannel <- hash
  }
  for {
    select {
      case fileUpdate := <- input:
        filename := path.Base(fileUpdate.path)
        if !fileUpdate.exists {
          if children[filename] != nil {
            log.Printf("Removed %s", filename)
            delete(children, filename)
            updateSelf()
          }
        } else {
          hash, err := storage.Configured().Put(fileUpdate.bytes)
          check(err)
          if children[filename] == nil || !bytes.Equal(hash, children[filename].Hash) {
            op := "Added"
            if children[filename] != nil { op = "Updated" }
            log.Printf("%s %s %d %#x", op, filename, fileUpdate.size, GetShortHexString(hash))
            children[filename] = &types.TreeEntry{Hash: hash}
            updateSelf()
          }
        }
      case mergeHash := <-mergeChannel:
        // This is not a merge but a destructive fast-forward
        _, commitBlob := GetBlob(mergeHash)
        if commitBlob.Commit == nil {
          log.Fatalf("Tried to merge commit %s but got %v instead", GetShortHexString(mergeHash), commitBlob)
        }
        _, treeBlob := GetBlob(commitBlob.Commit.Tree)
        tree := treeBlob.Tree
        log.Printf("Merging %s into tree (%d entries)", GetShortHexString(mergeHash), len(tree.Entries))
        children = map[string]*types.TreeEntry{}
        for _, entry := range tree.Entries {
          children[entry.Name] = entry
          _, fileblob := GetBlob(entry.Hash)
          file := fileblob.File
          ioutil.WriteFile(path.Join(rootPath, entry.Name), file.Bytes, 0644)
          log.Printf("Unpacked %s, %s", entry.Name, GetShortHexString(entry.Hash))
        }
    }
  }
}

type FileUpdate struct {
  bytes  []byte
  path   string
  exists bool
  size   int64
}


type FileEvent struct {
  path          string
  resultChannel chan FileUpdate
}

func processChange(inputChannel chan FileEvent) {
  for event := range inputChannel {
    statbuf, err := os.Stat(event.path)
    if err != nil {
      // The file was deleted or otherwise doesn't exist
      event.resultChannel <- FileUpdate{path: event.path, exists: false}
    } else {
      // Read the entire file and calculate its hash
      // XXX alternate path for large files?
      bytes, err := ioutil.ReadFile(event.path)
      check(err)
      // Send the update back to the tree's result channel
      event.resultChannel <- FileUpdate{bytes: bytes, path: event.path, exists: true, size: statbuf.Size()}
    }
  }
}

func debounce(output chan FileEvent, input chan FileEvent) {
  var waiting = map[string] bool {}
  var timeout_channel = make(chan FileEvent, 100)
  for {
    select {
      case in := <-input:
        if !waiting[in.path] {
          waiting[in.path] = true
          go func(_in FileEvent) {
              time.Sleep(5 * time.Millisecond)
              timeout_channel <- _in
          }(in)
        }
      case ready := <-timeout_channel:
        waiting[ready.path] = false
        output <- ready
    }
  }
}

func WatchTree(watchPath string, resultChannel chan FileUpdate) {
  watcher, _ := fsnotify.NewWatcher()
  watcher.Watch(watchPath)
  files, err := ioutil.ReadDir(watchPath)
  check(err)
  for _, file := range files {
    processChannel <- FileEvent{path.Join(watchPath, file.Name()), resultChannel}
  }
  for {
    select {
      case event := <-watcher.Event:
        if event.IsCreate() || event.IsModify() || event.IsDelete() || event.IsRename() {
          processChannel <- FileEvent{event.Name, resultChannel}
        } else {
          log.Fatal("unknown event type", event)
        }
      case error := <-watcher.Error:
        log.Fatal(error)
    }
  }
}

func WatchRevisions(commit *types.Commit, revisionChannel chan types.Hash, mergeChannel chan types.Hash) {
  branchReceiveChannel := make(chan types.BranchStatus, 10)
  subscription := types.BranchSubscription{Name: "master", ResponseChannel: branchReceiveChannel}
  types.BranchSubscribeChannel <- subscription
  var lastCommitHash types.Hash
  for {
    select {
      case newHash := <-revisionChannel:
        commit = &types.Commit{
          Text: "awesome",
          Tree: newHash,
          Parents: []types.Hash{}, // this needs the previous *commit* hash
        }
        if lastCommitHash != nil {
          commit.Parents = append(commit.Parents, lastCommitHash)
        }
        commitBytes, err := serializer.Configured().Marshal(&types.Blob{Commit: commit})
        check(err)
        commitHash, err := storage.Configured().Put(commitBytes)
        check(err)
        log.Printf("New branch revision: %s", GetShortHexString(commitHash))
        lastCommitHash = commitHash
        types.BranchUpdateChannel <- types.BranchStatus{Name: "master", Hash: commitHash}
      case newBranchStatus := <-branchReceiveChannel:
        if !bytes.Equal(commit.Tree, newBranchStatus.Hash) {
          // log.Printf("New remote revision: %s", GetShortHexString(newBranchStatus.Hash))
          mergeChannel <- newBranchStatus.Hash
        }
    }
  }
}

func MakeBranch(path string, previous *types.Commit, root *types.Tree) {
  revisionChannel := make(chan types.Hash, 10)
  mergeChannel := make(chan types.Hash, 10)
  // if root == nil {
  //   root =
  // }
  MakeEmptyTreeBlob(path, revisionChannel, mergeChannel)
  go WatchRevisions(&types.Commit{Tree: types.Hash{}}, revisionChannel, mergeChannel)
}

func StartProcessors() {
  var processImmChannel = make(chan FileEvent, 100)
  var WORKER_COUNT = 1
  for i := 0; i < WORKER_COUNT; i++ {
    go processChange(processImmChannel)
  }
  go debounce(processImmChannel, processChannel)
}
