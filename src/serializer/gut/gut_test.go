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

func ExampleUmarshalBranch() {
  s := Serializer{}
  blob, err := s.Unmarshal([]byte("5beebcdfedd26e654b88d2ce2d06fc1825e809d6"))
  check(err)
  fmt.Printf("%+v\n", blob.Branch)
  fmt.Printf("%+v\n", blob.File)
  // Output:
  // &{Name: Commit:[91 238 188 223 237 210 110 101 75 136 210 206 45 6 252 24 37 232 9 214]}
  // &{Bytes:[53 98 101 101 98 99 100 102 101 100 100 50 54 101 54 53 52 98 56 56 100 50 99 101 50 100 48 54 102 99 49 56 50 53 101 56 48 57 100 54]}
}

