
package blob

import (
  "fmt"
  "log"
  "../serializer"
  "../storage"
  "../types"
)

func check(err interface{}) {
  if err != nil {
    log.Fatal(err)
  }
}

// type Blob struct {
//   bytes  []byte
//   hash   Hash
// }

// type Commit struct {
//   root     *Blob
//   previous *Commit
// }

func GetBlob(hash types.Hash) ([]byte, *types.Blob) {
  bytes, err := storage.Configured().Get(hash)
  if err == nil {
    log.Printf("Found %s in cache", GetShortHexString(hash))
  } else {
    responseChannel := make(chan types.Hash)
    // XXX this should be more ... targetted
    // XXX also, there's a race condition between looking on disk
    // and subscribing to object reception.  I think.  Maybe not.
    // Maybe we'll just make a duplicate network request.
    log.Printf("Requesting %s", GetShortHexString(hash))
    types.BlobRequestChannel <- types.BlobRequest{Hash: hash, ResponseChannel: responseChannel}
    // XXX what about failure?  timeout?
    bytes = <-responseChannel
    log.Printf("Received %s", GetShortHexString(hash))
    storage.Configured().Put(bytes)
  }
  blob, err := serializer.Configured().Unmarshal(bytes)
  check(err)
  return bytes, blob
}

// func (blob *Blob) Hash() []byte {
//   if blob.hash == nil {
//     blob.hash = calculateHash(blob.bytes)
//   }
//   return blob.hash
// }

// func (blob *Blob) Bytes() []byte {
//   if blob.bytes == nil {
//     hash := blob.Hash()
//     if hash == nil {
//       log.Fatal("hash is nil")
//     }
//     bytes, err := ioutil.ReadFile(GetCachePath(hash))
//     check(err)
//     blob.bytes = bytes
//   }
//   return blob.bytes
// }

func GetShortHexString(bytes []byte) string {
  return GetHexString(bytes[:8])
}

func GetHexString(bytes []byte) string {
  return fmt.Sprintf("%#x", bytes)
}

// func (blob *Blob) HashString() string {
//   return GetHexString(blob.Hash())
// }

// func (blob *Blob) ShortHash() []byte {
//   return blob.Hash()[:8]
// }

// func (blob *Blob) ShortHashString() string {
//   return GetHexString(blob.ShortHash())
// }

// func MakeEmptyFileBlob() *Blob {
//   return &Blob{}
// }

// func MakeFileBlobFromHash(hash Hash) *Blob {
//   return &Blob{hash: hash}
// }

// func MakeFileBlobFromBytes(bytes []byte) *Blob {
//   return &Blob{bytes: bytes}
// }

func MakeEmptyTreeBlob(path string, revisionChannel chan types.Hash, mergeChannel chan types.Hash) *types.Tree {
  me := &types.Tree{}
  resultChannel := make(chan FileUpdate, 10)
  go MonitorTree(path, resultChannel, mergeChannel, revisionChannel)
  go WatchTree(path, resultChannel)
  return me
}
