
package gut

import (
  "bufio"
  "bytes"
  "encoding/hex"
  "errors"
  "fmt"
  "regexp"
  "strconv"
  "strings"
  "../../types"
)

type Serializer struct {}

func (s *Serializer) Unmarshal(data []byte) (*types.Blob, error) {
  // XXX these regexps are not complete...
  regexpTreeWhole := regexp.MustCompile("^(\\d{6} (blob|tree) [0-9a-f]{40}\\s[^\\n]+\\n)+$")
  regexpTreeLine := regexp.MustCompile("^(\\d{6}) (blob|tree) ([0-9a-f]{40})\\s([^\\n]+)$")
  regexpCommit := regexp.MustCompile("^tree [0-9a-f]{40}")
  regexpBranch := regexp.MustCompile("^[0-9a-f]{40}$")
  blob := &types.Blob{}
  if regexpBranch.Match(data) {
    fullText := bytes.NewBuffer(data).String()
    hash, _ := hex.DecodeString(fullText)
    blob.Branch = &types.Branch{Commit: hash}
  }
  if regexpTreeWhole.Match(data) {
    blob.Tree = &types.Tree{Entries: []*types.TreeEntry{}}
    fullText := bytes.NewBuffer(data).String()
    for _, line := range strings.Split(fullText, "\n") {
      submatches := regexpTreeLine.FindStringSubmatch(line)
      // XXX we really just want to exclude the last empty line
      if len(submatches) >= 3 {
        hash, _ := hex.DecodeString(submatches[3])
        flags, _ := strconv.ParseUint(submatches[1], 8, 32)
        entry := &types.TreeEntry{Hash: hash, Name: submatches[4], Flags: uint32(flags)}
        blob.Tree.Entries = append(blob.Tree.Entries, entry)
      }
    }
  }
  if regexpCommit.Match(data) {

  }
  blob.File = &types.File{Bytes: data}
  return blob, nil
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
