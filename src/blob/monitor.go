
package blob

import (
  "io/ioutil"
  "log"
  "os"
  "path"
  "time"
  "code.google.com/p/goprotobuf/proto"
  "github.com/howeyc/fsnotify"
  "../sharedpb"
  "../types"
)

var processChannel = make(chan FileEvent, 100) // debouncing

func (tree *Blob) MonitorTree(rootPath string, input chan FileUpdate, mergeChannel chan Hash, revisionChannel chan *Blob) {
  // XXX ideally, this would be a B-Tree with distributed caching
  var children = map[string]*Blob{}
  updateSelf := func() {
    pbtree := sharedpb.Tree{ Entries: []*sharedpb.TreeEntry{} }
    for name, blob := range children {
      Flags := uint32(0644)
      IsTree := false
      Name := name
      pbtree.Entries = append(pbtree.Entries, &sharedpb.TreeEntry{
        Hash: blob.Hash(),
        Flags: &Flags,
        IsTree: &IsTree,
        Name: &Name,
      })
    }
    tree.bytes, _ = proto.Marshal(&pbtree)
    tree.hash = nil
    tree.EnsureCached()
    revisionChannel <- tree
  }
  unpackSelf := func() {
    pbtree := &sharedpb.Tree{}
    err := proto.Unmarshal(tree.bytes, pbtree)
    check(err)
    children = map[string]*Blob{}
    log.Printf("Unpacking %d entries", len(pbtree.Entries))
    for _, entry := range pbtree.Entries {
      blob := GetBlob(entry.Hash)
      children[*entry.Name] = blob
      ioutil.WriteFile(path.Join(rootPath, *entry.Name), blob.Bytes(), 0644)
      log.Printf("Unpacked %s %s", *entry.Name, GetShortHexString(entry.Hash))
    }
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
          if children[filename] == nil ||
             children[filename].HashString() != fileUpdate.blob.HashString() {
            op := "Added"
            if children[filename] != nil { op = "Updated" }
            log.Printf("%s %s %d %#x", op, filename, fileUpdate.size, fileUpdate.blob.ShortHash())
            children[filename] = fileUpdate.blob
            updateSelf()
          }
        }
      case mergeHash := <-mergeChannel:
        blob := GetBlob(mergeHash)
        tree.bytes = blob.Bytes()
        tree.hash = blob.Hash()
        log.Printf("Merging %s into tree (%d bytes)", blob.ShortHashString(), len(tree.bytes))
        unpackSelf()
    }
  }
}

type FileUpdate struct {
  blob   *Blob
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
      event.resultChannel <- FileUpdate{MakeEmptyFileBlob(), event.path, false, 0}
    } else {
      // Read the entire file and calculate its hash
      // XXX alternate path for large files?
      bytes, err := ioutil.ReadFile(event.path)
      check(err)
      blob := MakeFileBlobFromBytes(bytes)
      blob.EnsureCached()
      // Send the update back to the tree's result channel
      event.resultChannel <- FileUpdate{blob, event.path, true, statbuf.Size()}
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

func (commit *Commit) WatchRevisions(revisionChannel chan *Blob, mergeChannel chan Hash) {
  branchReceiveChannel := make(chan types.BranchStatus, 10)
  subscription := types.BranchSubscription{Name: "master", ResponseChannel: branchReceiveChannel}
  types.BranchSubscribeChannel <- subscription
  for {
    select {
      case newTree := <-revisionChannel:
        log.Printf("New branch revision: %s", newTree.ShortHashString())
        name := "master"
        // message := sharedpb.Message{Branch: &sharedpb.Branch{Name: &name, Hash: newTree.Hash()}}
        types.BranchUpdateChannel <- types.BranchStatus{Name: name, Hash: newTree.Hash()}
        commit.root = newTree
      case newBranchStatus := <-branchReceiveChannel:
        // blob := &Blob{hash: newRemoteTreeHash}
        // log.Printf("New remote revision: %s", GetShortHexString(newRemoteTreeHash[:8]))
        mergeChannel <- newBranchStatus.Hash
    }
  }
}

func MakeBranch(path string, previous *Commit, root *Blob) *Commit {
  revisionChannel := make(chan *Blob, 10)
  mergeChannel := make(chan Hash, 10)
  if root == nil {
    root = MakeTreeBlob(path, revisionChannel, mergeChannel)
  }
  me := Commit{
    root: root,
    previous: previous,
  }
  go me.WatchRevisions(revisionChannel, mergeChannel)
  return &me
}

func StartProcessors() {
  var processImmChannel = make(chan FileEvent, 100)
  var WORKER_COUNT = 1
  for i := 0; i < WORKER_COUNT; i++ {
    go processChange(processImmChannel)
  }
  go debounce(processImmChannel, processChannel)
}
