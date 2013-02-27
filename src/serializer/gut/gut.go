
package gut

import (
  "bufio"
  "bytes"
  "errors"
  "fmt"
  "regexp"
  "../../types"
)

type Serializer struct {}

func (s *Serializer) Unmarshal(bytes []byte) (*types.Blob, error) {
  // XXX these regexps are not complete...
  regexpTreeWhole := regexp.MustCompile("^((\\d{6}) (blob|tree) ([0-9a-f]{40})\\t([^\\n]+)\\n)+$")
  // regexpTreeLine := regexp.MustCompile("(\\d{6}) (blob|tree) ([0-9a-f]{40})\\t([^\\n]+)\\n")
  regexpCommit := regexp.MustCompile("^tree [0-9a-f]{40}")
  regexpBranch := regexp.MustCompile("^[0-9a-f]{40}$")
  blob := &types.Blob{}
  if regexpBranch.Match(bytes) {

  } else if regexpTreeWhole.Match(bytes) {

  } else if regexpCommit.Match(bytes) {

  } else {
    blob.File = &types.File{Bytes: bytes}
  }
  return nil, nil
}

func GetHexString(bytes []byte) string {
  return fmt.Sprintf("%#x", bytes)
}

func (s *Serializer) Marshal(blob *types.Blob) ([]byte, error) {
  buffer := &bytes.Buffer{}
  writer := bufio.NewWriter(buffer)
  if blob.Branch != nil {
    writer.Write([]byte(GetHexString(blob.Branch.Commit)))
  } else if blob.Tree != nil {
    for _, entry := range blob.Tree.Entries {
      writer.Write([]byte(fmt.Sprintf("100644 blob %s\t%s\n", GetHexString(entry.Hash), entry.Name)))
    }
  } else if blob.Commit != nil {
    writer.Write([]byte(fmt.Sprintf("tree %s\n", GetHexString(blob.Commit.Tree))))
    // writer.Write(fmt.Sprintf("parent %s\n", ))
    writer.Write([]byte("author Dan Tillberg <dan@tillberg.us> 1361046035 +0000\n"))
    writer.Write([]byte("committer Dan Tillberg <dan@tillberg.us> 1361046035 +0000\n"))
    writer.Write([]byte("\nshared commit\n"))
  } else if blob.File != nil {
    writer.Write(blob.File.Bytes)
  } else {
    return nil, errors.New("No blob field defined")
  }
  writer.Flush()
  return buffer.Bytes(), nil
}
