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

type Blob struct {
  bytes  []byte
  hash   []byte

  // non-empty for top-level files and trees:
  path string

  // non-empty for non-leaf files and trees:
  segments []*Blob

  // lazily populated for tree roots:
  // XXX ideally, this would be a B-Tree with distributed caching
  children map[string]*Blob

  // non-nil for everything except share roots:
  parent *Blob

  // non-nil only on share roots (except for the "initial commit",
  // which is also nil)
  previous *Blob

  is_tree bool
  is_file bool

  // XXX for top level file: mode flags, timestamps?
  // XXX for top level tree: ?
}

// type Tree interface {

//   NavigateTo(path string) Tree
//   Remove(path string)
//   Update(blob Blob)
// }

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

func MakeEmptyFileBlob(path string) *Blob {
  return &Blob{path: path, is_tree: false, is_file: true}
}

func MakeFileBlob(path string, hash []byte) *Blob {
  return &Blob{path: path, hash: hash, is_tree: false, is_file: true}
}

func WatchTree(watchPath string, updateChannel chan FileEvent, resultChannel chan FileUpdate) {
  watcher, _ := fsnotify.NewWatcher()
  watcher.Watch(watchPath)
  var files, _ = ioutil.ReadDir(watchPath)
  for _, file := range files {
    updateChannel <- FileEvent{path.Join(watchPath, file.Name()), resultChannel}
  }
  for {
    select {
      case event := <-watcher.Event:
        if event.IsCreate() || event.IsModify() || event.IsDelete() || event.IsRename() {
          updateChannel <- FileEvent{event.Name, resultChannel}
        } else {
          log.Fatal("unknown event type", event)
        }
      case error := <-watcher.Error:
        log.Fatal(error)
    }
  }
}

func MakeTreeBlob(path string, parent *Blob, updateChannel chan FileEvent) *Blob {
  me := Blob{path: path, parent: parent, is_tree: true, is_file: false}
  resultChannel := make(chan FileUpdate, 10)
  go notificationReporter(resultChannel)
  go WatchTree(me.path, updateChannel, resultChannel)
  return &me
}

func MakeShareRootBlob(path string, previous *Blob, updateChannel chan FileEvent) *Blob {
  me := MakeTreeBlob(path, nil, updateChannel)
  me.previous = previous
  return me
}

type FileUpdate struct {
  Blob
  exists bool
  size   int64
}

func calculateHash(bytes []byte) []byte {
  SHA256_SALT_BEFORE := []byte{'s', 'h', 'a', 'r', 'e', 'd', '('}
  SHA256_SALT_AFTER := []byte{')'}
  h := sha256.New()
  h.Write(SHA256_SALT_BEFORE)
  h.Write(bytes)
  h.Write(SHA256_SALT_AFTER)
  return h.Sum([]byte{})
}

func notificationReporter(input chan FileUpdate) {
  for op := range input {
    fmt.Printf("%s %d %#x\n", op.path, op.size, op.Hash());
  }
}

type FileEvent struct {
  path          string
  resultChannel chan FileUpdate
}

func processChange(cacheRoot string, inputChannel chan FileEvent) {
  for event := range inputChannel {
    statbuf, err := os.Stat(event.path)
    if err != nil {
      event.resultChannel <- FileUpdate{*MakeEmptyFileBlob(event.path), false, 0}
    } else {
      // Read the entire file and calculate its hash
      // XXX alternate path for large files?
      bytes, err := ioutil.ReadFile(event.path)
      if err != nil {

        log.Printf("Error reading `%s`: %s", event.path, err);
      } else {
        hash := calculateHash(bytes)

        // Save a copy in the cache if we don't already have one
        hashString := fmt.Sprintf("%#x", hash)
        cachePath := path.Join(cacheRoot, hashString)
        _, err := os.Stat(cachePath)
        if err != nil {
          ioutil.WriteFile(cachePath, bytes, 0644)
        }

        // Send the update back to the tree's result channel
        event.resultChannel <- FileUpdate{
          *MakeFileBlob(event.path, hash),
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
  fmt.Println("Started.")
  go restartOnChange()
  os.Mkdir(*cache_root, 0755)
  var processChannel = make(chan FileEvent, 100)
  var debounceChannel = make(chan FileEvent, 100)
  var WORKER_COUNT = 1
  for i := 0; i < WORKER_COUNT; i++ {
    go processChange(*cache_root, processChannel)
  }
  go debounce(processChannel, debounceChannel)

  MakeShareRootBlob(*watch_target, nil, debounceChannel)

  for {
    time.Sleep(time.Second)
  }
}
