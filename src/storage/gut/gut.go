
package gut

import (
  "bufio"
  "bytes"
  "crypto/sha1"
  "encoding/hex"
  "log"
  "io"
  "io/ioutil"
  "os"
  "path"
  "compress/zlib"
  "../../types"
)

type Storage struct {
  RootPath string
}

func (s *Storage) getCachePath(hash types.Hash) string {
  hashString := hex.EncodeToString(hash)
  return path.Join(s.RootPath, "objects", hashString[:2], hashString[2:])
}

func calculateHash(bytes []byte) types.Hash {
  h := sha1.New()
  h.Write(bytes)
  return h.Sum([]byte{})
}

func (s *Storage) Get(hash types.Hash) (data []byte, err error) {
  cachePath := s.getCachePath(hash)
  _, err = os.Stat(cachePath)
  if err == nil {
    dataCompressed, _ := ioutil.ReadFile(cachePath)
    bufferCompressed := bytes.NewBuffer(dataCompressed)
    r, _ := zlib.NewReader(bufferCompressed)
    bufferUncompressed := bytes.Buffer{}
    writerUncompressed := bufio.NewWriter(&bufferUncompressed)
    io.Copy(writerUncompressed, r)
    r.Close()
    writerUncompressed.Flush()
    data = bufferUncompressed.Bytes()
  }
  return data, err
}

func (s *Storage) Put(data []byte) (hash types.Hash, err error) {
  hash = calculateHash(data)
  cachePath := s.getCachePath(hash)
  _, err = os.Stat(cachePath)
  if err != nil {
    os.MkdirAll(path.Dir(cachePath), 0755)
    // bufferUncompressed := bytes.NewBuffer(data)
    // readerUncompressed := bufio.NewReader(bufferUncompressed)
    bufferCompressed := bytes.Buffer{}
    // writerCompressed := bufio.NewWriter(&bufferCompressed)
    w := zlib.NewWriter(&bufferCompressed)
    // io.Copy(w, bufferCompressed)
    w.Write(data)
    w.Flush()
    w.Close()
    // writerCompressed.Close()
    newData := bufferCompressed.Bytes()
    log.Printf("zlib %s from %d to %d bytes", hex.EncodeToString(hash), len(data), len(newData))
    text := bytes.NewBuffer(data).String()
    log.Printf("before: \n%s", text)
    err = ioutil.WriteFile(cachePath, newData, 0644)
  }
  return hash, err
}

func (*Storage) PutRef(name string, hash types.Hash) error {
  return nil
}

func (*Storage) GetRef(name string) (types.Hash, error) {
  return nil, nil
}
