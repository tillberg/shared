
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

func TestBasic(t *testing.T) {
    setup := test.SetUp()
    ioutil.WriteFile("/tmp/sync1/testfile", []byte{}, 0644)
    AssertContents(t, time.Second, "/tmp/sync2/testfile", "")
    test.TearDown(setup)
}
