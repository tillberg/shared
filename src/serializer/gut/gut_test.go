package gut

import (
  "fmt"
  "testing"
)

func check(t *testing.T, err error) {
  if err != nil {
    t.Fatal(err)
  }
}

func TestUnmarshalBranch(t *testing.T) {
  s := Serializer{}
  blob, err := s.Unmarshal([]byte("5beebcdfedd26e654b88d2ce2d06fc1825e809d6"))
  check(t, err)
  fmt.Print(blob)

}
