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
}

type FileBlob struct {
  Blob
  path   string
  parent *TreeBlob
  // mode flags
  // timestamps?
}

type TreeBlob struct {
  Blob
  path string
  children map[string]*PathedBlob
  // other properties?
}

type PathedBlob interface {
  IsFile() bool
  Hash()   []byte
}

type FileBlobUpdate struct {
  FileBlob
  exists bool
  size   int64
}

func (blob Blob) Hash() []byte {
  if blob.hash == nil {
    blob.hash = calculateHash(blob.bytes)
  }
  return blob.hash
}

func (b FileBlob) IsFile() bool { return true }
func (b TreeBlob) IsFile() bool { return false }

func calculateHash(bytes []byte) []byte {
  SHA256_SALT_BEFORE := []byte{'s', 'h', 'a', 'r', 'e', 'd', '('}
  SHA256_SALT_AFTER := []byte{')'}
  h := sha256.New()
  h.Write(SHA256_SALT_BEFORE)
  h.Write(bytes)
  h.Write(SHA256_SALT_AFTER)
  return h.Sum([]byte{})
}

func notificationReporter(input chan FileBlobUpdate) {
  for op := range input {
    fmt.Printf("%s %d %#x\n", op.path, op.size, op.Hash());
  }
}

func processChange(output chan FileBlobUpdate, path_channel chan string) {
  for path := range path_channel {
    statbuf, err := os.Stat(path)
    if err != nil {
      output <- FileBlobUpdate{FileBlob{Blob{nil, nil}, path, nil}, false, 0}
    } else {
      bytes, err := ioutil.ReadFile(path)
      if err != nil { panic(err) }
      output <- FileBlobUpdate{FileBlob{Blob{bytes, nil}, path, nil}, true, statbuf.Size()}
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
  select {
    case <- watcher.Event:
      os.Exit(0)
  }
}

func main() {
  flag.Parse()
  fmt.Println("Started.")
  go restartOnChange()
  var report_channel = make(chan FileBlobUpdate, 100)
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
