
package gut

import (
  "bufio"
  "bytes"
  "compress/zlib"
  "crypto/sha1"
  "encoding/hex"
  "errors"
  "fmt"
  "io"
  "io/ioutil"
  "log"
  "os"
  "path"
  "../../serializer"
  "../../types"
)

type Storage struct {
  RootPath string
}

func (s *Storage) getCachePath(hash types.Hash) string {
  hashString := hex.EncodeToString(hash)
  return path.Join(s.RootPath, "objects", hashString[:2], hashString[2:])
}

func (s *Storage) Deflate(in []byte) []byte {
  var b bytes.Buffer
  w := zlib.NewWriter(&b)
  w.Write(in)
  w.Close()
  return b.Bytes()
}

func (s *Storage) Inflate(in []byte) ([]byte, error) {
  b := bytes.NewBuffer(in)
  r, err := zlib.NewReader(b)
  if err != nil {
    return nil, err
  }
  bufferUncompressed := bytes.Buffer{}
  writerUncompressed := bufio.NewWriter(&bufferUncompressed)
  io.Copy(writerUncompressed, r)
  defer r.Close()
  writerUncompressed.Flush()
  return bufferUncompressed.Bytes(), nil
}

func calculateHash(bytes []byte) types.Hash {
  h := sha1.New()
  h.Write(bytes)
  return h.Sum([]byte{})
}

func (s *Storage) Get(hash types.Hash) (blob types.Blob, err error) {
  cachePath := s.getCachePath(hash)
  _, err = os.Stat(cachePath)
  if err != nil { return blob, err }
  compressed, err := ioutil.ReadFile(cachePath)
  if err != nil { return blob, err }
  data, err := s.Inflate(compressed)
  if err != nil {
    return blob, errors.New(fmt.Sprintf("Error (%s) while inflating object: %s", err, cachePath))
  }
  blob, err = serializer.Configured().Unmarshal(data)
  return blob, err
}

func (s *Storage) Put(blob types.Blob) (hash types.Hash, err error) {
  data, err := serializer.Configured().Marshal(blob)
  hash = calculateHash(data)
  compressed := s.Deflate(data)
  cachePath := s.getCachePath(hash)
  os.MkdirAll(path.Dir(cachePath), 0755)
  log.Printf("Saving %s to cache (%d bytes)", hex.EncodeToString(hash)[:8], len(data))
  err = ioutil.WriteFile(cachePath, compressed, 0644)
  return hash, err
}

func (s *Storage) PutRef(name string, hash types.Hash) error {
  refPath := path.Join(s.RootPath, "refs", "heads", name)
  os.MkdirAll(path.Dir(refPath), 0755)
  ioutil.WriteFile(refPath, []byte(fmt.Sprintf("%s\n", hex.EncodeToString(hash))), 0644)
  return nil
}

func (s *Storage) GetRef(name string) (types.Hash, error) {
  return nil, nil
}
