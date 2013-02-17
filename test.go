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

var watch_target *string = flag.String("watch", "_sync", "The directory to keep an eye on")

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

  // non-nil for everything except root trees:
  parent *Blob

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

func processChange(output chan FileUpdate, path_channel chan string) {
  for path := range path_channel {
    statbuf, err := os.Stat(path)
    if err != nil {
      output <- FileUpdate{Blob{nil, nil, path, nil, nil, nil, false, true}, false, 0}
    } else {
      bytes, err := ioutil.ReadFile(path)
      if err != nil { panic(err) }
      output <- FileUpdate{Blob{bytes, nil, path, nil, nil, nil, false, true}, true, statbuf.Size()}
    }
  }
}

func debounce(output chan string, input chan string) {
  var waiting = map[string] bool {}
  var timeout_channel = make(chan string, 100)
  for {
    select {
      case in := <-input:
        if !waiting[in] {
          waiting[in] = true
          go func(_in string) {
              time.Sleep(100 * time.Millisecond)
              timeout_channel <- _in
          }(in)
        }
      case ready := <-timeout_channel:
        waiting[ready] = false
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
  var report_channel = make(chan FileUpdate, 100)
  var update_channel = make(chan string, 100)
  var debounce_channel = make(chan string, 100)
  go notificationReporter(report_channel)
  var WORKER_COUNT = 1
  for i := 0; i < WORKER_COUNT; i++ {
    go processChange(report_channel, update_channel)
  }
  go debounce(update_channel, debounce_channel)
  watchpath := *watch_target
  watcher, _ := fsnotify.NewWatcher()
  watcher.Watch(watchpath)
  var files, _ = ioutil.ReadDir(watchpath)
  for _, file := range files {
    update_channel <- path.Join(watchpath, file.Name())
  }
  for {
    select {
      case event := <-watcher.Event:
        if event.IsCreate() || event.IsModify() || event.IsDelete() || event.IsRename() {
          debounce_channel <- event.Name
        } else {
          log.Fatal("unknown event type", event)
        }
      case error := <-watcher.Error:
        log.Fatal(error)
    }
  }
}
