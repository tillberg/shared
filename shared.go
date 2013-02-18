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
  "code.google.com/p/goprotobuf/proto"
  "./sharedpb"
)

var watch_target *string = flag.String("watch", "_sync", "The directory to sync")
var cache_root *string = flag.String("cache", "_cache", "Directory to keep cache of objects")

var processChannel = make(chan FileEvent, 100) // debouncing

type Blob struct {
  bytes  []byte
  hash   []byte

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

func calculateHash(bytes []byte) []byte {
  h := sha256.New()
  h.Write(SHA256_SALT_BEFORE)
  h.Write(bytes)
  h.Write(SHA256_SALT_AFTER)
  return h.Sum([]byte{})
}

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

func (blob Blob) EnsureCached() {
  // Save a copy in the cache if we don't already have one
  hashString := blob.HashString()
  cachePath := path.Join(*cache_root, hashString[:2], hashString[2:])
  _, err := os.Stat(cachePath)
  if err != nil {
    os.MkdirAll(path.Join(*cache_root, hashString[:2]), 0755)
    ioutil.WriteFile(cachePath, blob.bytes, 0644)
    log.Printf("Cached %s", hashString[:16])
  }
}

func MakeEmptyFileBlob() *Blob {
  return &Blob{is_tree: false, is_file: true}
}

func MakeFileBlobFromHash(hash []byte) *Blob {
  return &Blob{hash: hash, is_tree: false, is_file: true}
}

func MakeFileBlobFromBytes(bytes []byte) *Blob {
  return &Blob{bytes: bytes, is_tree: false, is_file: true}
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

func (tree Blob) MonitorTree(input chan FileUpdate) {
  // XXX ideally, this would be a B-Tree with distributed caching
  var children = map[string]*Blob{}
  updateSelf := func() {
    pbtree := sharedpb.Tree{}

    tree.bytes, _ = proto.Marshal(&pbtree)
    tree.hash = nil
    tree.EnsureCached()
    tree.revisionChannel <- &tree
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
            log.Printf("%s %s %d %#x\n", op, filename, fileUpdate.size, fileUpdate.blob.ShortHash())
            children[filename] = fileUpdate.blob
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
      if err != nil {
        log.Printf("Error reading `%s`: %s", event.path, err);
      } else {
        blob := MakeFileBlobFromBytes(bytes)
        blob.EnsureCached()
        // Send the update back to the tree's result channel
        event.resultChannel <- FileUpdate{
          blob,
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
  watcher.Watch("shared.go")
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
    go processChange(processImmChannel)
  }
  go debounce(processImmChannel, processChannel)

  MakeBranch(*watch_target, nil, nil)

  select {}
}
