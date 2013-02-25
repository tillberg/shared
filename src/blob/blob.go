
package blob

import (
  "crypto/sha256"
  "fmt"
  "io/ioutil"
  "log"
  "os"
  "path"
  "../sharedpb"
  "../types"
)

func check(err interface{}) {
  if err != nil {
    log.Fatal(err)
  }
}


type Hash []byte

type Blob struct {
  bytes  []byte
  hash   Hash
}

type Commit struct {
  root     *Blob
  previous *Commit
}

var CacheRoot = ""

func GetCachePath(hash Hash) string {
  hashString := GetHexString(hash)
  return path.Join(CacheRoot, hashString[:2], hashString[2:])
}

func GetBlob(hash Hash) *Blob {
  cachePath := GetCachePath(hash)
  _, err := os.Stat(cachePath)
  var blob *Blob
  if err == nil {
    log.Printf("Found %s in cache", GetShortHexString(hash))
    blob = MakeFileBlobFromHash(hash)
  } else {
    responseChannel := make(chan []byte)
    // XXX this should be more ... targetted
    // XXX also, there's a race condition between looking on disk
    // and subscribing to object reception.  I think.  Maybe not.
    // Maybe we'll just make a duplicate network request.
    log.Printf("Requesting %s", GetShortHexString(hash))
    types.BlobRequestChannel <- types.BlobRequest{Hash: hash, ResponseChannel: responseChannel}
    bytes := <-responseChannel
    blob = MakeFileBlobFromBytes(bytes)
    log.Printf("Received %s", GetShortHexString(hash))
    blob.EnsureCached()
  }
  return blob
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

func GetShortHexString(bytes []byte) string {
  return GetHexString(bytes[:8])
}

func GetHexString(bytes []byte) string {
  return fmt.Sprintf("%#x", bytes)
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

func MakeTreeBlob(path string, revisionChannel chan *Blob, mergeChannel chan Hash) *Blob {
  me := Blob{}
  resultChannel := make(chan FileUpdate, 10)
  go me.MonitorTree(path, resultChannel, mergeChannel, revisionChannel)
  go WatchTree(path, resultChannel)
  return &me
}

func SendObject(hash Hash, dest chan *sharedpb.Message) {
  blob := GetBlob(hash)
  dest <- &sharedpb.Message{Object: &sharedpb.Object{Hash: blob.Hash(), Object: blob.Bytes()}}
}
