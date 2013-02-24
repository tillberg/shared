
package shared_test

import (
    "testing"
    "./test"
    "io/ioutil"
    "time"
)

func AssertContents(t *testing.T, timeout time.Duration, path string, contents string) {
    start := time.Now()
    for {
        bytes, err := ioutil.ReadFile(path)
        if err == nil && string(bytes) == contents {
            return
        }
        if (time.Since(start) > timeout) {
            t.Fatalf("%s failed to contain `%s`", path, contents)
        }
        time.Sleep(time.Millisecond)
    }
}

var fastTimeout = 100 * time.Millisecond
var timeout = 250 * time.Millisecond

func TestBasic(t *testing.T) {
    setup := test.SetUp()
    defer test.TearDown(setup)
    ioutil.WriteFile("/tmp/sync1/testfile", []byte{}, 0644)
    AssertContents(t, timeout, "/tmp/sync2/testfile", "")
}

func TestContents(t* testing.T) {
    setup := test.SetUp()
    defer test.TearDown(setup)
    ioutil.WriteFile("/tmp/sync1/testfile", []byte("hello"), 0644)
    AssertContents(t, timeout, "/tmp/sync2/testfile", "hello")
}

func TestTwo(t* testing.T) {
    setup := test.SetUp()
    defer test.TearDown(setup)
    ioutil.WriteFile("/tmp/sync1/testfile", []byte("hello"), 0644)
    ioutil.WriteFile("/tmp/sync1/testfile2", []byte("hello to you"), 0644)
    AssertContents(t, timeout, "/tmp/sync2/testfile", "hello")
    AssertContents(t, fastTimeout, "/tmp/sync2/testfile2", "hello to you")
}

func TestTwoBefore(t* testing.T) {
    test.Cleanup()
    ioutil.WriteFile("/tmp/sync1/testfile", []byte("hello"), 0644)
    ioutil.WriteFile("/tmp/sync1/testfile2", []byte("hello to you"), 0644)
    setup := test.Start()
    defer test.TearDown(setup)
    AssertContents(t, timeout, "/tmp/sync2/testfile", "hello")
    AssertContents(t, fastTimeout, "/tmp/sync2/testfile2", "hello to you")
}
