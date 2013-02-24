package main

import (
  "flag"
  "fmt"
  "os"
  "os/signal"
  "log"
  "time"
  "path"
  "net"
  "bufio"
  "crypto/sha256"
  "io/ioutil"
  "github.com/howeyc/fsnotify"
  "code.google.com/p/goprotobuf/proto"
  "./sharedpb"
  "./network"
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

type BlobRequest struct {
  hash            []byte
  responseChannel chan *Blob
}

type BranchSubscription struct {
  branch          string
  responseChannel chan *sharedpb.Message
}

var processChannel = make(chan FileEvent, 100) // debouncing
var blobRequestChannel = make(chan BlobRequest, 10)
var branchStatusChannel = make(chan *sharedpb.Message, 10)
var subscribeChannel = make(chan chan *sharedpb.Message, 10)
var objectReceiveChannel = make(chan *Blob, 100)
var branchReceiveChannel = make(chan []byte, 10)
var branchSubscribeChannel = make(chan *BranchSubscription, 10)

type Hash []byte

type Blob struct {
  bytes  []byte
  hash   Hash
}

type Commit struct {
  root     *Blob
  previous *Commit
}

func GetShortHexString(bytes []byte) string {
  return GetHexString(bytes[:8])
}

func GetHexString(bytes []byte) string {
  return fmt.Sprintf("%#x", bytes)
}

func GetCachePath(hash Hash) string {
  hashString := GetHexString(hash)
  return path.Join(*cache_root, hashString[:2], hashString[2:])
}

func GetBlob(hash Hash) *Blob {
  cachePath := GetCachePath(hash)
  _, err := os.Stat(cachePath)
  var blob *Blob
  if err == nil {
    log.Printf("Found %s in cache", GetShortHexString(hash))
    blob = MakeFileBlobFromHash(hash)
  } else {
    responseChannel := make(chan *Blob)
    // XXX this should be more ... targetted
    // XXX also, there's a race condition between looking on disk
    // and subscribing to object reception.  I think.  Maybe not.
    // Maybe we'll just make a duplicate network request.
    log.Printf("Requesting %s", GetShortHexString(hash))
    blobRequestChannel <- BlobRequest{hash: hash, responseChannel: responseChannel}
    blob = <-responseChannel
    log.Printf("Received %s", GetShortHexString(hash))
    blob.EnsureCached()
  }
  return blob
}

func calculateHash(bytes []byte) []byte {
  h := sha256.New()
  h.Write(SHA256_SALT_BEFORE)
  h.Write(bytes)
  h.Write(SHA256_SALT_AFTER)
  return h.Sum([]byte{})
}

func (blob *Blob) Hash() []byte {
  if blob.hash == nil {
    blob.hash = calculateHash(blob.bytes)
  }
  return blob.hash
}

func (blob *Blob) Bytes() []byte {
  if blob.bytes == nil {
    hash := blob.Hash()
    if hash == nil {
      log.Fatal("hash is nil")
    }
    bytes, err := ioutil.ReadFile(GetCachePath(hash))
    check(err)
    blob.bytes = bytes
  }
  return blob.bytes
}

func (blob *Blob) HashString() string {
  return GetHexString(blob.Hash())
}

func (blob *Blob) ShortHash() []byte {
  return blob.Hash()[:8]
}

func (blob *Blob) ShortHashString() string {
  return GetHexString(blob.ShortHash())
}

func (blob *Blob) EnsureCached() {
  // Save a copy in the cache if we don't already have one
  cachePath := GetCachePath(blob.Hash())
  _, err := os.Stat(cachePath)
  if err != nil {
    os.MkdirAll(path.Dir(cachePath), 0755)
    ioutil.WriteFile(cachePath, blob.bytes, 0644)
    log.Printf("Cached %s", blob.ShortHashString())
  }
}

func MakeEmptyFileBlob() *Blob {
  return &Blob{}
}

func MakeFileBlobFromHash(hash Hash) *Blob {
  return &Blob{hash: hash}
}

func MakeFileBlobFromBytes(bytes []byte) *Blob {
  return &Blob{bytes: bytes}
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

func (tree *Blob) MonitorTree(input chan FileUpdate, mergeChannel chan Hash, revisionChannel chan *Blob) {
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
      ioutil.WriteFile(path.Join(*watch_target, *entry.Name), blob.Bytes(), 0644)
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

func MakeTreeBlob(path string, revisionChannel chan *Blob, mergeChannel chan Hash) *Blob {
  me := Blob{}
  resultChannel := make(chan FileUpdate, 10)
  go me.MonitorTree(resultChannel, mergeChannel, revisionChannel)
  go WatchTree(path, resultChannel)
  return &me
}

func (commit *Commit) WatchRevisions(revisionChannel chan *Blob, mergeChannel chan Hash) {
  for {
    select {
      case newTree := <-revisionChannel:
        log.Printf("New branch revision: %s", newTree.ShortHashString())
        name := "master"
        message := sharedpb.Message{Branch: &sharedpb.Branch{Name: &name, Hash: newTree.Hash()}}
        branchStatusChannel <- &message
        commit.root = newTree
      case newRemoteTreeHash := <-branchReceiveChannel:
        // blob := &Blob{hash: newRemoteTreeHash}
        // log.Printf("New remote revision: %s", GetShortHexString(newRemoteTreeHash[:8]))
        mergeChannel <- newRemoteTreeHash
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

func ArbitBlobRequests() {
  servicers := []chan *sharedpb.Message{}
  subscribers := map[string][]chan *Blob{}
  for {
    select {
      case subscriber := <-subscribeChannel:
        servicers = append(servicers, subscriber)
      case request := <-blobRequestChannel:
        for _, servicer := range servicers {
          servicer <- &sharedpb.Message{HashRequest: request.hash}
        }
        hashString := GetHexString(request.hash)
        log.Printf("Waiting for %s", GetShortHexString(request.hash))
        if subscribers[hashString] == nil {
          subscribers[hashString] = []chan *Blob{}
        }
        subscribers[hashString] = append(subscribers[hashString], request.responseChannel)
      case object := <-objectReceiveChannel:
        // log.Printf("Forwarding %s", GetHexString(object.Hash()))
        for _, subscriber := range subscribers[GetHexString(object.Hash())] {
          subscriber <- object
        }
    }
  }
}

func ArbitBranchStatus() {
  subscribers := map[string][]chan *sharedpb.Message{}
  statuses := map[string]*sharedpb.Message{}
  for {
    select {
      case subscription := <-branchSubscribeChannel:
        branch := subscription.branch
        if subscribers[branch] == nil {
          subscribers[branch] = []chan *sharedpb.Message{}
        }
        subscribers[branch] = append(subscribers[branch], subscription.responseChannel)
        if statuses[branch] != nil {
          subscription.responseChannel <- statuses[branch]
        }
      case branchStatus := <-branchStatusChannel:
        branch := *branchStatus.Branch.Name
        statuses[branch] = branchStatus
        for _, subscriber := range subscribers[branch] {
          subscriber <- branchStatus
        }
    }
  }
}

func SendObject(hash Hash, dest chan *sharedpb.Message) {
  blob := GetBlob(hash)
  dest <- &sharedpb.Message{Object: &sharedpb.Object{Hash: blob.Hash(), Object: blob.Bytes()}}
}

func MessageString(m *sharedpb.Message) string {
  if m.HashRequest != nil {
    return fmt.Sprintf("{HashRequest: %s}", GetShortHexString(m.HashRequest))
  } else if m.Branch != nil {
    return fmt.Sprintf("{Branch: %s -> %s}", *m.Branch.Name, GetShortHexString(m.Branch.Hash))
  } else if m.Object != nil {
    return fmt.Sprintf("{Object: %s -> %d bytes}", GetShortHexString(m.Object.Hash), len(m.Object.Object))
  } else if m.SubscribeBranch != nil {
    return fmt.Sprintf("{Subscribe: %s}", *m.SubscribeBranch)
  }
  log.Fatal("Unknown message: ", m)
  return ""
}

func connOutgoing(conn *net.TCPConn, outbox chan *sharedpb.Message) {
  s := "master"
  outbox<-&sharedpb.Message{SubscribeBranch: &s}
  subscribeChannel <- outbox
  writer := bufio.NewWriter(conn)
  for {
    message := <- outbox
    err := network.SendMessage(message, writer)
    check(err)
    log.Printf("Sent %s", MessageString(message))
  }
}

func connIncoming(conn *net.TCPConn, outbox chan *sharedpb.Message) {
  reader := bufio.NewReader(conn)
  for {
    _, message, err := network.ReceiveMessage(reader)
    check(err)
    log.Printf("Received %s", MessageString(message))
    if message.HashRequest != nil {
      go SendObject(message.HashRequest, outbox)
    } else if message.Object != nil {
      objectReceiveChannel <- MakeFileBlobFromBytes(message.Object.Object)
    } else if message.Branch != nil {
      branchReceiveChannel <- message.Branch.Hash
    } else if message.SubscribeBranch != nil {
      branchSubscribeChannel <- &BranchSubscription{*message.SubscribeBranch, outbox}
    } else {
      log.Fatal("Unknown incoming message", MessageString(message))
    }
  }
}

func startConnections(conn *net.TCPConn) {
  outbox := make(chan *sharedpb.Message, 10)
  go connOutgoing(conn, outbox)
  connIncoming(conn, outbox)
}

func makeConnection(remoteAddr *net.TCPAddr) {
  start := time.Now()
  for {
    conn, err := net.DialTCP("tcp", nil, remoteAddr)
    if err != nil {
      if time.Since(start) > time.Second {
        log.Fatal(err)
      }
      time.Sleep(10 * time.Millisecond)
      continue
    }
    log.Printf("Connected to %s.", remoteAddr)
    startConnections(conn)
  }
}

func handleConnection(conn *net.TCPConn) {
  log.Printf("Connection received from %s", conn.RemoteAddr().String())
  startConnections(conn)
}

func ListenForConnections(ln *net.TCPListener) {
  for {
    conn, err := ln.AcceptTCP()
    if err != nil {
      log.Print(err)
      continue
    }
    go handleConnection(conn)
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
  config, err := conf.ReadConfigFile("shared.ini")
  check(err)
  _, err = config.GetString("main", "apikey")
  check(err)
  go restartOnChange()
  var processImmChannel = make(chan FileEvent, 100)
  var WORKER_COUNT = 1
  for i := 0; i < WORKER_COUNT; i++ {
    go processChange(processImmChannel)
  }
  go debounce(processImmChannel, processChannel)
  go ArbitBranchStatus()
  go ArbitBlobRequests()

  MakeBranch(*watch_target, nil, nil)

  listen_addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", *listen_port));
  check(err)
  ln, err := net.ListenTCP("tcp", listen_addr)
  check(err)
  defer ln.Close()
  log.Printf("Listening on port %d.", *listen_port)
  // XXX omg kludge.  Need to figure out how to properly negotiate
  // unique full-duplex P2P connections.
  if *listen_port == 9252 {
    remote_addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:9251");
    check(err)
    go makeConnection(remote_addr)
  }
  go ListenForConnections(ln)
  interrupt := make(chan os.Signal, 2)
  signal.Notify(interrupt, os.Interrupt)
  <-interrupt
}
