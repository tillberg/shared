package gut

import (
  "bytes"
  "encoding/hex"
  "fmt"
  "strings"
  "testing"
  "../../types"
)

func check(err error) {
  if err != nil {
    panic(err)
  }
}

func exampleTreeString() string {
  s := (
    "100644 blob ce770ef0fd9f274ea9e102910396d57956edd325\t.gitignore\n" +
    "100755 blob 094dce1db4f2dc0b25068a61fab98c746e5b4834\tdev.sh\n" +
    "040000 tree 6116b1ca7b8e616cd3997b4acac9e5731b34073e\tproto\n" +
    "040000 tree c78ce6725d1cda9b01fd4ad4701f5776ed27f4a0\tsrc\n" +
    "432176 blob 8f2a73dfc460145d671908bd6497cc6ff765d9d3\ttest.sh\n")
  return fmt.Sprintf("tree %d\000%s", len(s), s)
}

func ExampleSerializer_Unmarshal_Tree() {
  s := Serializer{}
  blob, err := s.Unmarshal(Deflate([]byte(exampleTreeString())))
  check(err)
  fmt.Printf("%+v\n", blob.Tree.Entries[0])
  fmt.Printf("%+v\n", blob.Tree.Entries[1])
  fmt.Printf("%+v\n", blob.Tree.Entries[2])
  fmt.Printf("%+v\n", blob.Tree.Entries[3])
  fmt.Printf("%+v\n", blob.Tree.Entries[4])
  fmt.Println(blob.Tree.Entries[4].Flags == 0432176)
  // Output:
  // &{Hash:[206 119 14 240 253 159 39 78 169 225 2 145 3 150 213 121 86 237 211 37] Name:.gitignore Flags:33188}
  // &{Hash:[9 77 206 29 180 242 220 11 37 6 138 97 250 185 140 116 110 91 72 52] Name:dev.sh Flags:33261}
  // &{Hash:[97 22 177 202 123 142 97 108 211 153 123 74 202 201 229 115 27 52 7 62] Name:proto Flags:16384}
  // &{Hash:[199 140 230 114 93 28 218 155 1 253 74 212 112 31 87 118 237 39 244 160] Name:src Flags:16384}
  // &{Hash:[143 42 115 223 196 96 20 93 103 25 8 189 100 151 204 111 247 101 217 211] Name:test.sh Flags:144510}
  // true
}

func exampleCommitString() string {
  s := (
    `tree c68f49cedb6379a88f36a20ed5c6ca8bf735e73b
parent 5beebcdfedd26e654b88d2ce2d06fc1825e809d6
parent e673cec71f4dbbe6e765f3f448f705a4c78d157f
author Dan Tillberg <dan@tillberg.us> 1361048340 +0000
committer Dan Tillberg <dan@tillberg.us> 1361048340 +0000

Read all files in folder on startup
`)
  return fmt.Sprintf("commit %d\000%s", len(s), s)
}


func ExampleSerializer_Unmarshal_Commit() {
  s := Serializer{}
  blob, err := s.Unmarshal(Deflate([]byte(exampleCommitString())))
  check(err)
  fmt.Printf("%+v\n", blob.Commit.Tree)
  fmt.Printf("%+v\n", blob.Commit.Parents)
  fmt.Printf("%+v\n", blob.Commit.Text)
  // Output:
  // [198 143 73 206 219 99 121 168 143 54 162 14 213 198 202 139 247 53 231 59]
  // [[91 238 188 223 237 210 110 101 75 136 210 206 45 6 252 24 37 232 9 214] [230 115 206 199 31 77 187 230 231 101 243 244 72 247 5 164 199 141 21 127]]
  // author Dan Tillberg <dan@tillberg.us> 1361048340 +0000
  // committer Dan Tillberg <dan@tillberg.us> 1361048340 +0000
  //
  // Read all files in folder on startup
}

func ExampleSerializer_Marshal_Tree() {
  s := Serializer{}
  hash, _ := hex.DecodeString("5beebcdfedd26e654b88d2ce2d06fc1825e809d6")
  hash2, _ := hex.DecodeString("e673cec71f4dbbe6e765f3f448f705a4c78d157f")
  entries := []*types.TreeEntry{
    &types.TreeEntry{Hash: hash, Name: "bob", Flags: 040123},
    &types.TreeEntry{Hash: hash2, Name: "susan", Flags: 0123456},
  }
  data, err := s.Marshal(&types.Blob{Tree: &types.Tree{Entries: entries}})
  check(err)
  data, err = Inflate(data)
  check(err)
  text := bytes.NewBuffer(data).String()
  text = strings.Replace(text, "\t", "  ", -1)
  text = strings.Replace(text, "\000", "{ZERO}", -1)
  fmt.Println(text)
  // Output:
  // tree 116{ZERO}040123 tree 5beebcdfedd26e654b88d2ce2d06fc1825e809d6  bob
  // 123456 blob e673cec71f4dbbe6e765f3f448f705a4c78d157f  susan
}

func TestSerializer_Marshal_Tree(t *testing.T) {
  s := Serializer{}
  blob, err := s.Unmarshal(Deflate([]byte(exampleTreeString())))
  check(err)
  data, err := s.Marshal(blob)
  check(err)
  data, err = Inflate(data)
  check(err)
  text := bytes.NewBuffer(data).String()
  if text != exampleTreeString() {
    t.Fatalf("Got this:\n%s\nExpected this:\n%s\n", text, exampleTreeString())
  }
}

func TestSerializer_Marshal_Commit(t *testing.T) {
  s := Serializer{}
  blob, err := s.Unmarshal(Deflate([]byte(exampleCommitString())))
  check(err)
  data, err := s.Marshal(blob)
  check(err)
  data, err = Inflate(data)
  check(err)
  text := bytes.NewBuffer(data).String()
  if text != exampleCommitString() {
    t.Fatalf("Got this:\n%s\nExpected this:\n%s\n", text, exampleCommitString())
  }
}
