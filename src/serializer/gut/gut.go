
package gut

import (
  "bufio"
  "bytes"
  "compress/zlib"
  "encoding/hex"
  "errors"
  "fmt"
  "io"
  "regexp"
  "strconv"
  "strings"
  "../../types"
)

type Serializer struct {}

func Deflate(in []byte) []byte {
  var b bytes.Buffer
  w := zlib.NewWriter(&b)
  w.Write(in)
  w.Close()
  return b.Bytes()
}

func Inflate(in []byte) ([]byte, error) {
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

func (s *Serializer) Unmarshal(data []byte) (blob *types.Blob, err error) {
  // XXX these regexps are not complete...
  // regexpTreeWhole := regexp.MustCompile(`^(\d{6} (blob|tree) [0-9a-f]{40}\s[^\n]+\n)+$`)
  // XXX bug in regexp? [\s\S] 20 times is not the same as [\s\S]{20}
  regexpTreeEntry := regexp.MustCompile(
    `(\d+) (.+?)\000([\s\S][\s\S][\s\S][\s\S][\s\S][\s\S][\s\S][\s\S][\s\S][\s\S][\s\S][\s\S][\s\S][\s\S][\s\S][\s\S][\s\S][\s\S][\s\S])`)
  regexpCommit := regexp.MustCompile(`^tree ([0-9a-f]{40})\n((parent [0-9a-f]{40}\n)*)((.|\n)+)$`)
  regexpCommitLine := regexp.MustCompile(`^(tree|parent) ([0-9a-f]{40})$`)
  // regexpBranch := regexp.MustCompile("^[0-9a-f]{40}$")
  blob = &types.Blob{}
  regexpHeader := regexp.MustCompile(`^((\w+) \d+\000)`)
  data, err = Inflate(data)
  if err != nil {
    return nil, errors.New(fmt.Sprintf("Failed to inflate Unmarshal input (%d bytes): %s", len(data), err))
  }
  submatchHeader := regexpHeader.FindSubmatch(data)
  if submatchHeader == nil {
    return nil, errors.New("Could not read git object header.")
  }
  t := bytes.NewBuffer(submatchHeader[2]).String()
  data = data[len(submatchHeader[1]):]
  if t == "tree" {
    blob.Tree = &types.Tree{Entries: []*types.TreeEntry{}}
    for _, submatches := range regexpTreeEntry.FindAllSubmatch(data, -1) {
      flagsString := bytes.NewBuffer(submatches[1]).String()
      flags, _ := strconv.ParseUint(flagsString, 8, 32)
      nameString := bytes.NewBuffer(submatches[2]).String()
      entry := &types.TreeEntry{Hash: submatches[3], Name: nameString, Flags: uint32(flags)}
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

func (s *Serializer) Marshal(blob *types.Blob) ([]byte, error) {
  buffer := &bytes.Buffer{}
  writer := bufio.NewWriter(buffer)
  var t string
  if blob.Tree != nil {
    t = "tree"
    for _, entry := range blob.Tree.Entries {
      flagsString := fmt.Sprintf("%06o", entry.Flags)
      writer.Write([]byte(fmt.Sprintf("%s %s\000", flagsString, entry.Name)))
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
  // Write header
  w.Write([]byte(fmt.Sprintf("%s %d\000", t, len(data))))
  w.Write(data)
  w.Flush()
  return Deflate(b.Bytes()), nil
}
