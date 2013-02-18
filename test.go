package main

import (
  "flag"
  "fmt"
  "os"
  "log"
  "time"
  "path"
  "crypto/sha256"
  "io/ioutil"
  "github.com/howeyc/fsnotify"
)

var watch_target *string = flag.String("watch", "_sync", "The directory to sync")
var cache_root *string = flag.String("cache", "_cache", "Directory to keep cache of objects")

var processChannel = make(chan FileEvent, 100) // debouncing

type Blob struct {
  bytes  []byte
  hash   []byte

  // non-empty for top-level files and trees:
  // path string

  // XXX ideally, this would be a B-Tree with distributed caching
  children map[string]*Blob

  // non-empty for non-leaf files and trees:
  // segments []*Blob

  // Channel to communicate with map of children
  childrenChannel chan string

  // Channel to send self-updates to.  This maybe should move to
  // a more flexible pub/sub type deal in the future.
  revisionChannel chan *Blob

  // non-nil for everything except share roots:
  parent *Blob

  is_tree bool
  is_file bool

  // XXX for tree entries: mode flags
}

type Commit struct {
  root     *Blob
  previous *Commit
}

func (b Blob) FirstParent(f func(*Blob) bool) *Blob {
  for p := b.parent; p != nil; p = p.parent {
    if f(p) {
      return p
    }
  }
  return nil
}

func (b Blob) ShareRoot() *Blob {
  return b.FirstParent(func(b *Blob) bool {
    return b.IsShareRoot()
  })
}

func (b Blob) Root() *Blob {
  root := b.FirstParent(func(b *Blob) bool {
    return b.IsFile() || b.IsTree()
  })
  if root == nil {
    panic("Blob has no root")
  }
  return root
}

func (b Blob) IsShareRoot()   bool { return b.parent == nil }
func (b Blob) IsTree()        bool { return b.is_tree }
func (b Blob) IsTreeSegment() bool { return b.Root().IsTree() }
func (b Blob) IsFile()        bool { return b.is_file }
func (b Blob) IsFileSegment() bool { return b.Root().IsFile() }

func (blob Blob) Hash() []byte {
  if blob.hash == nil {
    blob.hash = calculateHash(blob.bytes)
  }
  return blob.hash
}

func (blob Blob) HashString() string {
  return fmt.Sprintf("%#x", blob.Hash())
}

func (blob Blob) ShortHash() []byte {
  return blob.Hash()[:8]
}

func (blob Blob) ShortHashString() string {
  return fmt.Sprintf("%#x", blob.ShortHash())
}

func MakeEmptyFileBlob() *Blob {
  return &Blob{is_tree: false, is_file: true}
}

func MakeFileBlob(hash []byte) *Blob {
  return &Blob{hash: hash, is_tree: false, is_file: true}
}

func WatchTree(watchPath string, resultChannel chan FileUpdate) {
  watcher, _ := fsnotify.NewWatcher()
  watcher.Watch(watchPath)
  var files, _ = ioutil.ReadDir(watchPath)
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

func CloneTreeBlob(blob *Blob) *Blob {
  children := map[string]*Blob{}
  for k, v := range blob.children {
    children[k] = v
  }
  return &Blob{
    parent: blob.parent,
    is_tree: blob.is_tree,
    is_file: blob.is_file,
    children: children,
    revisionChannel: blob.revisionChannel,
    childrenChannel: make(chan string, 10),
  }
}

func (tree Blob) MonitorTree(input chan FileUpdate) {
  // var children = map[string]*Blob{}
  updateSelf := func() {
    tree.bytes = []byte(fmt.Sprint(tree.children))
    tree.revisionChannel <- &tree
  }
  for {
    select {
      case fileUpdate := <- input:
        filename := path.Base(fileUpdate.path)
        if !fileUpdate.exists {
          if tree.children[filename] != nil {
            log.Printf("Removed %s", filename)
            delete(tree.children, filename)
            updateSelf()
          }
        } else {
          if tree.children[filename] == nil ||
             tree.children[filename].HashString() != fileUpdate.blob.HashString() {
            op := "Added"
            if tree.children[filename] != nil { op = "Updated" }
            log.Printf("%s %s %d %#x\n", op, filename, fileUpdate.size, fileUpdate.blob.ShortHash())
            tree.children[filename] = fileUpdate.blob
            updateSelf()
          }
        }
      // case lookup := <- tree.childrenChannel:
    }
  }
}

func MakeTreeBlob(path string, parent *Blob, revisionChannel chan *Blob) *Blob {
  me := Blob{
    parent: parent,
    is_tree: true,
    is_file: false,
    childrenChannel: make(chan string, 10),
    revisionChannel: revisionChannel,
    children: make(map[string]*Blob),
  }
  resultChannel := make(chan FileUpdate, 10)
  go me.MonitorTree(resultChannel)
  go WatchTree(path, resultChannel)
  return &me
}

func (commit *Commit) WatchRevisions(revisionChannel chan *Blob) {
  for {
    select {
      case newTree := <-revisionChannel:
        log.Printf("New branch revision: %s", newTree.ShortHashString())
        commit.root = newTree
    }
  }
}

func MakeBranch(path string, previous *Commit, root *Blob) *Commit {
  revisionChannel := make(chan *Blob, 10)
  if root == nil {
    root = MakeTreeBlob(path, nil, revisionChannel)
  }
  me := Commit{
    root: root,
    previous: previous,
  }
  go me.WatchRevisions(revisionChannel)
  return &me
}

type FileUpdate struct {
  blob   *Blob
  path   string
  exists bool
  size   int64
}

var SHA256_SALT_BEFORE = []byte{'s', 'h', 'a', 'r', 'e', 'd', '('}
var SHA256_SALT_AFTER = []byte{')'}

func calculateHash(bytes []byte) []byte {
  h := sha256.New()
  h.Write(SHA256_SALT_BEFORE)
  h.Write(bytes)
  h.Write(SHA256_SALT_AFTER)
  return h.Sum([]byte{})
}

type FileEvent struct {
  path          string
  resultChannel chan FileUpdate
}

func processChange(cacheRoot string, inputChannel chan FileEvent) {
  for event := range inputChannel {
    statbuf, err := os.Stat(event.path)
    if err != nil {
      // The file was deleted or otherwise doesn't exist
      event.resultChannel <- FileUpdate{MakeEmptyFileBlob(), event.path, false, 0}
    } else {
      // Read the entire file and calculate its hash
      // XXX alternate path for large files?
      bytes, err := ioutil.ReadFile(event.path)
      if err != nil {
        log.Printf("Error reading `%s`: %s", event.path, err);
      } else {
        // Save a copy in the cache if we don't already have one
        hash := calculateHash(bytes)
        hashString := fmt.Sprintf("%#x", hash)
        cachePath := path.Join(cacheRoot, hashString[:2], hashString[2:])
        _, err := os.Stat(cachePath)
        if err != nil {
          os.MkdirAll(path.Join(cacheRoot, hashString[:2]), 0755)
          ioutil.WriteFile(cachePath, bytes, 0644)
          log.Printf("Cached %s", hashString[:16])
        }

        // Send the update back to the tree's result channel
        event.resultChannel <- FileUpdate{
          MakeFileBlob(hash),
          event.path,
          true,
          statbuf.Size(),
        }
      }
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
              time.Sleep(100 * time.Millisecond)
              timeout_channel <- _in
          }(in)
        }
      case ready := <-timeout_channel:
        waiting[ready.path] = false
        output <- ready
    }
  }
}

func restartOnChange() {
  watcher, _ := fsnotify.NewWatcher()
  watcher.Watch("test.go")
  <-watcher.Event
  os.Exit(0)
}

func main() {
  flag.Parse()
  log.Println("Started.")
  go restartOnChange()
  var processImmChannel = make(chan FileEvent, 100)
  var WORKER_COUNT = 1
  for i := 0; i < WORKER_COUNT; i++ {
    go processChange(*cache_root, processImmChannel)
  }
  go debounce(processImmChannel, processChannel)

  MakeBranch(*watch_target, nil, nil)

  select {}
}
