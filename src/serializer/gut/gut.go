
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

func (s *Serializer) Unmarshal(data []byte) (blob types.Blob, err error) {
  // regexpTreeWhole := regexp.MustCompile(`^(\d{6} (blob|tree) [0-9a-f]{40}\s[^\n]+\n)+$`)
  regexpTreeEntry := regexp.MustCompile(`^(\d+) (.+?)\000`)
  regexpCommit := regexp.MustCompile(`^tree ([0-9a-f]{40})\n((parent [0-9a-f]{40}\n)*)((.|\n)+)$`)
  regexpCommitLine := regexp.MustCompile(`^(tree|parent) ([0-9a-f]{40})$`)
  // regexpBranch := regexp.MustCompile("^[0-9a-f]{40}$")
  blob = types.Blob{}
  regexpHeader := regexp.MustCompile(`^((\w+) \d+\000)`)
  submatchHeader := regexpHeader.FindSubmatch(data)
  if submatchHeader == nil {
    return blob, errors.New("Could not read git object header.")
  }
  t := bytes.NewBuffer(submatchHeader[2]).String()
  data = data[len(submatchHeader[1]):]
  if t == "tree" {
    blob.Tree = &types.Tree{Entries: []*types.TreeEntry{}}
    for {
      submatch := regexpTreeEntry.FindSubmatch(data);
      if submatch == nil {
        if len(data) != 0 {
          err = errors.New(fmt.Sprintf("Error reading tree.  %d entries found, %d bytes remain.",
            len(blob.Tree.Entries), len(data)))
        }
        break
      }
      // Remove the bytes from data that we just matched
      data = data[len(submatch[0]):]
      flagsString := bytes.NewBuffer(submatch[1]).String()
      flags, _ := strconv.ParseUint(flagsString, 8, 32)
      nameString := bytes.NewBuffer(submatch[2]).String()
      // The next 20 bytes in data are the 160-bit SHA1 hash
      entry := &types.TreeEntry{Hash: data[:20], Name: nameString, Flags: uint32(flags)}
      data = data[20:]
      blob.Tree.Entries = append(blob.Tree.Entries, entry)
    }
  } else if t == "commit" {
    fullText := bytes.NewBuffer(data).String()
    commitSubmatches := regexpCommit.FindStringSubmatch(fullText)
    hash, _ := hex.DecodeString(commitSubmatches[1])
    blob.Commit = &types.Commit{Parents: []types.Hash{}, Tree: hash, Text: commitSubmatches[4]}
    for _, line := range strings.Split(commitSubmatches[2], "\n") {
      if line != "" {
        submatches := regexpCommitLine.FindStringSubmatch(line)
        hash, _ = hex.DecodeString(submatches[2])
        blob.Commit.Parents = append(blob.Commit.Parents, hash)
      }
    }
  } else if t == "blob" {
    blob.File = &types.File{Bytes: data}
  } else {
    err = errors.New(fmt.Sprintf("Unknown object type: %s", t))
  }
  return blob, err
}

func GetHexString(bytes []byte) string {
  return fmt.Sprintf("%#x", bytes)
}

func (s *Serializer) Marshal(blob types.Blob) ([]byte, error) {
  buffer := &bytes.Buffer{}
  writer := bufio.NewWriter(buffer)
  var t string
  if blob.Tree != nil {
    t = "tree"
    for _, entry := range blob.Tree.Entries {
      flagsString := fmt.Sprintf("%o", entry.Flags)
      writer.Write([]byte(fmt.Sprintf("%s %s", flagsString, entry.Name)))
      writer.Write([]byte{0})
      writer.Write(entry.Hash)
    }
  } else if blob.Commit != nil {
    t = "commit"
    writer.Write([]byte(fmt.Sprintf("tree %s\n", GetHexString(blob.Commit.Tree))))
    for _, parent := range blob.Commit.Parents {
      writer.Write([]byte(fmt.Sprintf("parent %s\n", GetHexString(parent))))
    }
    writer.Write([]byte(blob.Commit.Text))
  } else if blob.File != nil {
    t = "blob"
    writer.Write(blob.File.Bytes)
  } else {
    return nil, errors.New("No blob field defined")
  }
  writer.Flush()
  data := buffer.Bytes()
  b := &bytes.Buffer{}
  w := bufio.NewWriter(b)
  // Write header, and rewrite the actual data after it
  w.Write([]byte(fmt.Sprintf("%s %d\000", t, len(data))))
  w.Write(data)
  w.Flush()
  return b.Bytes(), nil
}
