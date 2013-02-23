
package main

import (
    "./test"
    "log"
    "io/ioutil"
    "time"
)

func main() {
    setup := test.SetUp()

    time.Sleep(500 * time.Millisecond)
    ioutil.WriteFile("/tmp/sync1/testfile", []byte{}, 0644)
    time.Sleep(500 * time.Millisecond)
    _, err := ioutil.ReadFile("/tmp/sync2/testfile")
    if err != nil {
        log.Fatal(err)
    }

    test.TearDown(setup)
}
