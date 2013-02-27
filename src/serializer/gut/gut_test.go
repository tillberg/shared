package gut

import (
  "fmt"
  // "testing"
)

func check(err error) {
  if err != nil {
    panic(err)
  }
}

func ExampleSerializer_Unmarshal_Branch() {
  s := Serializer{}
  blob, err := s.Unmarshal([]byte("5beebcdfedd26e654b88d2ce2d06fc1825e809d6"))
  check(err)
  fmt.Printf("%+v\n", blob.Branch)
  fmt.Printf("%+v\n", blob.File)
  // Output:
  // &{Name: Commit:[91 238 188 223 237 210 110 101 75 136 210 206 45 6 252 24 37 232 9 214]}
  // &{Bytes:[53 98 101 101 98 99 100 102 101 100 100 50 54 101 54 53 52 98 56 56 100 50 99 101 50 100 48 54 102 99 49 56 50 53 101 56 48 57 100 54]}
}

func ExampleSerializer_Unmarshal_Tree() {
  s := Serializer{}
  blob, err := s.Unmarshal([]byte(`100644 blob ce770ef0fd9f274ea9e102910396d57956edd325 .gitignore
100755 blob 094dce1db4f2dc0b25068a61fab98c746e5b4834  dev.sh
040000 tree 6116b1ca7b8e616cd3997b4acac9e5731b34073e  proto
040000 tree c78ce6725d1cda9b01fd4ad4701f5776ed27f4a0  src
432176 blob 8f2a73dfc460145d671908bd6497cc6ff765d9d3  test.sh
`))
  check(err)
  fmt.Printf("%+v\n", blob.Tree.Entries[0])
  fmt.Printf("%+v\n", blob.Tree.Entries[1])
  fmt.Printf("%+v\n", blob.Tree.Entries[2])
  fmt.Printf("%+v\n", blob.Tree.Entries[3])
  fmt.Printf("%+v\n", blob.Tree.Entries[4])
  fmt.Println(blob.Tree.Entries[4].Flags == 0432176)
  // Output:
  // &{Hash:[206 119 14 240 253 159 39 78 169 225 2 145 3 150 213 121 86 237 211 37] Name:.gitignore Flags:33188}
  // &{Hash:[9 77 206 29 180 242 220 11 37 6 138 97 250 185 140 116 110 91 72 52] Name: dev.sh Flags:33261}
  // &{Hash:[97 22 177 202 123 142 97 108 211 153 123 74 202 201 229 115 27 52 7 62] Name: proto Flags:16384}
  // &{Hash:[199 140 230 114 93 28 218 155 1 253 74 212 112 31 87 118 237 39 244 160] Name: src Flags:16384}
  // &{Hash:[143 42 115 223 196 96 20 93 103 25 8 189 100 151 204 111 247 101 217 211] Name: test.sh Flags:144510}
  // true
}
