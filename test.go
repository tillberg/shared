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

type FileOp struct {
  filename string
  size int64
  hash []byte
  contents []byte
}

func notificationReporter(input chan FileOp) {
  for op := range input {
    fmt.Printf("%s %d %#x\n", op.filename, op.size, op.hash);
  }
}

func getHash(bytes []byte) []byte {
  SHA256_SALT_BEFORE := []byte{'s', 'h', 'a', 'r', 'e', 'd', '('}
  SHA256_SALT_AFTER := []byte{')'}
  h := sha256.New()
  h.Write(SHA256_SALT_BEFORE)
  h.Write(bytes)
  h.Write(SHA256_SALT_AFTER)
  return h.Sum([]byte{})
}

func processChange(output chan FileOp, path chan string) {
  for p := range path {
    statbuf, err := os.Stat(p)
    if err != nil {
      output <- FileOp{p, -1, nil, nil}
    } else {
      bytes, err := ioutil.ReadFile(p)
      if err != nil { panic(err) }
      output <- FileOp{p, statbuf.Size(), getHash(bytes), bytes}
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
  var report_channel = make(chan FileOp, 100)
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
        switch {
          case event.IsCreate() || event.IsModify() || event.IsDelete() || event.IsRename():
            debounce_channel <- event.Name
          default:
            log.Fatal("unknown event type", event)
        }
      case error := <-watcher.Error:
        log.Fatal(error)
    }
  }
}
