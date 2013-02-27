
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
  // regexpTreeWhole := regexp.MustCompile(`^(\d{6} (blob|tree) [0-9a-f]{40}\s[^\n]+\n)+$`)
  regexpTreeLine := regexp.MustCompile(`^(\d{6}) (blob|tree) ([0-9a-f]{40})\s([^\n]+)$`)
  regexpCommit := regexp.MustCompile(`^tree ([0-9a-f]{40})\n((parent [0-9a-f]{40}\n)*)((.|\n)+)$`)
  regexpCommitLine := regexp.MustCompile(`^(tree|parent) ([0-9a-f]{40})$`)
  // regexpBranch := regexp.MustCompile("^[0-9a-f]{40}$")
  blob := &types.Blob{}
  regexpHeader := regexp.MustCompile(`^((\w+) \d+\000)`)
  submatchHeader := regexpHeader.FindSubmatch(data)
  t := bytes.NewBuffer(submatchHeader[1]).String()
  data = data[len(submatchHeader[0]):]

  if t == "branch" { // XXX branch?  this won't work...
    fullText := bytes.NewBuffer(data).String()
    hash, _ := hex.DecodeString(fullText)
    blob.Branch = &types.Branch{Commit: hash}
  }
  if t == "tree" {
    blob.Tree = &types.Tree{Entries: []*types.TreeEntry{}}
    fullText := bytes.NewBuffer(data).String()
    for _, line := range strings.Split(fullText, "\n") {
      // XXX we really just want to exclude the last empty line
      if line != "" {
        submatches := regexpTreeLine.FindStringSubmatch(line)
        hash, _ := hex.DecodeString(submatches[3])
        flags, _ := strconv.ParseUint(submatches[1], 8, 32)
        entry := &types.TreeEntry{Hash: hash, Name: submatches[4], Flags: uint32(flags)}
        blob.Tree.Entries = append(blob.Tree.Entries, entry)
      }
    }
  }
  if t == "commit" {
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
  }
  if t == "blob" {
    blob.File = &types.File{Bytes: data}
  }
  return blob, nil
}

func GetHexString(bytes []byte) string {
  return fmt.Sprintf("%#x", bytes)
}

func (s *Serializer) Marshal(blob *types.Blob) ([]byte, error) {
  buffer := &bytes.Buffer{}
  writer := bufio.NewWriter(buffer)
  var t string
  if blob.Branch != nil {
    writer.Write([]byte(GetHexString(blob.Branch.Commit)))
  } else if blob.Tree != nil {
    t = "tree"
    for _, entry := range blob.Tree.Entries {
      flagsString := fmt.Sprintf("%06o", entry.Flags)
      // e.g. 040000 is a tree.  we don't record the "tree"/"blob" flag, but git does
      typeString := "blob"
      if entry.Flags >> 9 == 040 {
        typeString = "tree"
      }
      hexHash := GetHexString(entry.Hash)
      // flagsString := strings.Replace(strconv.FormatUint(uint64(entry.Flags), 8), " ", "0", -1)
      writer.Write([]byte(fmt.Sprintf("%s %s %s\t%s\n", flagsString, typeString, hexHash, entry.Name)))
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
  if t != "" {
    // Write header
    w.Write([]byte(fmt.Sprintf("%s %d\000", t, len(data))))
  }
  w.Write(data)
  w.Flush()
  return b.Bytes(), nil
}
