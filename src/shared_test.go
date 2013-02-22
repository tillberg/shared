
package shared

import "testing"
import "os/exec"
import "time"
import "io/ioutil"

func Launch(cachePath string, syncPath string, port string) {
    cmd := exec.Cmd{
        Path: "go",
        Args: []string{"run", "src/shared.go", "--watch", syncPath," --cache", cachePath, "--port",  port},
    }
    cmd.Run()
}

func TestSync(_ *testing.T) {
    exec.Command("rm", "-rf", "/tmp/sync1").Run()
    exec.Command("mkdir", "/tmp/sync1").Run()
    exec.Command("rm", "-rf", "/tmp/sync2").Run()
    exec.Command("mkdir", "/tmp/sync2").Run()

    go Launch("/tmp/cache1", "/tmp/sync1", "9251")
    go Launch("/tmp/cache2", "/tmp/sync2", "9252")
    time.Sleep(time.Second)
    ioutil.WriteFile("/tmp/sync1/testfile", []byte{}, 0644)
    time.Sleep(time.Second)
    ioutil.ReadFile("/tmp/sync2/testfile")
}
