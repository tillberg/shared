
package shared_test

import (
  "os"
  "path"
  "testing"
  "time"
  "./test"
  "io/ioutil"
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

func WriteFile(filepath string, contents string) {
  os.MkdirAll(path.Dir(filepath), 0755)
  ioutil.WriteFile(filepath, []byte(contents), 0644)
}

func TestBasic(t *testing.T) {
  setup := test.SetUp()
  defer test.TearDown(setup)
  WriteFile("/tmp/sync1/testfile", "")
  AssertContents(t, timeout, "/tmp/sync2/testfile", "")
}

func TestContents(t* testing.T) {
  setup := test.SetUp()
  defer test.TearDown(setup)
  WriteFile("/tmp/sync1/testfile", "hello")
  AssertContents(t, timeout, "/tmp/sync2/testfile", "hello")
}

func TestTwo(t* testing.T) {
  setup := test.SetUp()
  defer test.TearDown(setup)
  WriteFile("/tmp/sync1/testfile", "hello")
  WriteFile("/tmp/sync1/testfile2", "hello to you")
  AssertContents(t, timeout, "/tmp/sync2/testfile", "hello")
  AssertContents(t, fastTimeout, "/tmp/sync2/testfile2", "hello to you")
}

func TestTwoBefore(t* testing.T) {
  test.Cleanup()
  WriteFile("/tmp/sync1/testfile", "hello")
  WriteFile("/tmp/sync1/testfile2", "hello to you")
  setup := test.Start()
  defer test.TearDown(setup)
  AssertContents(t, timeout, "/tmp/sync2/testfile", "hello")
  AssertContents(t, fastTimeout, "/tmp/sync2/testfile2", "hello to you")
}

func TestTwoConnectDuring(t* testing.T) {
  test.Cleanup()
  setup := test.Init()
  test.StartA(setup)
  test.StartB(setup)
  defer test.TearDown(setup)
  WriteFile("/tmp/sync1/testfile", "hello")
  WriteFile("/tmp/sync1/testfile2", "hello to you")
  test.ConnectBA()
  AssertContents(t, timeout, "/tmp/sync2/testfile", "hello")
  AssertContents(t, fastTimeout, "/tmp/sync2/testfile2", "hello to you")
}

func TestSingleRevision(t* testing.T) {
  setup := test.SetUp()
  defer test.TearDown(setup)
  WriteFile("/tmp/sync1/testfile", "hello")
  AssertContents(t, timeout, "/tmp/sync2/testfile", "hello")
  WriteFile("/tmp/sync1/testfile", "hello to you")
  AssertContents(t, timeout, "/tmp/sync2/testfile", "hello to you")
}

func TestMultipleRevisionLateStartup(t* testing.T) {
  test.Cleanup()
  WriteFile("/tmp/sync1/testfile", "hello")
  WriteFile("/tmp/sync1/testfile", "hello to you")
  setup := test.Start()
  defer test.TearDown(setup)
  AssertContents(t, timeout, "/tmp/sync2/testfile", "hello to you")
  WriteFile("/tmp/sync1/testfile", "hello")
  AssertContents(t, timeout, "/tmp/sync2/testfile", "hello")
}

// func TestMerge(t* testing.T) {
//   test.Cleanup()
//   setup := test.Start()
//   WriteFile("/tmp/sync1/testfile", "hello\nmy name is\nbob\n")
//   AssertContents(t, timeout, "/tmp/sync2/testfile", "hello\nmy name is\nbob\n")
//   test.Stop(setup)
//   WriteFile("/tmp/sync1/testfile", "hello\nsusan\nmy name is\nbob\n")
//   WriteFile("/tmp/sync2/testfile", "hello\nmy name is\npeter\n")
//   setup = test.Start()
//   defer test.TearDown(setup)
//   AssertContents(t, timeout, "/tmp/sync1/testfile", "hello\nsusan\nmy name is\npeter\n")
//   AssertContents(t, fastTimeout, "/tmp/sync2/testfile", "hello\nsusan\nmy name is\npeter\n")
// }
