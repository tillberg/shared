
package test

import (
    "os"
    "os/exec"
    "log"
    "fmt"
    "io"
    "bufio"
)

func ReadLines(id string, pipe io.Reader) {
    r := bufio.NewReader(pipe)
    line, err := r.ReadString('\n')
    for err == nil {
        fmt.Printf("%s: %s", id, line)
        line, err = r.ReadString('\n')
    }
}

func Launch(id string, cachePath string, syncPath string, port string, setup *TestSetup) {
    cmd := exec.Cmd{
        Path: "../shared",
        Args: []string{"shared", "--watch", syncPath, "--cache", cachePath, "--port",  port},
    }
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        log.Fatal(err)
    }
    stderr, err := cmd.StderrPipe()
    if err != nil {
        log.Fatal(err)
    }
    defer stdout.Close()
    defer stderr.Close()
    err = cmd.Start()
    if err != nil {
        log.Fatal(err)
    }
    go ReadLines(id, stdout)
    go ReadLines(id, stderr)
    setup.ready<-"ready"
    <-setup.quit
    cmd.Process.Signal(os.Interrupt)
    cmd.Wait()
    setup.ready<-"exited"
}

type TestSetup struct {
    ready chan string
    quit chan string
}

func CleanDir(path string) {
    exec.Command("/bin/rm", "-rf", path).Run()
    exec.Command("/bin/mkdir", path).Run()
}

func Cleanup() {
    CleanDir("/tmp/cache1")
    CleanDir("/tmp/cache2")
    CleanDir("/tmp/sync1")
    CleanDir("/tmp/sync2")
}

func Init() *TestSetup {
    return &TestSetup{ready: make(chan string), quit: make(chan string)}
}

func StartA(setup *TestSetup) {
    go Launch("A", "/tmp/cache1", "/tmp/sync1", "9251", setup)
    <-setup.ready
}

func StartB(setup *TestSetup) {
    go Launch("B", "/tmp/cache2", "/tmp/sync2", "9252", setup)
    <-setup.ready
}

func Start() *TestSetup {
    setup := Init()
    StartA(setup)
    StartB(setup)
    return setup
}

func SetUp() *TestSetup {
    Cleanup()
    return Start()
}

func TearDown(setup *TestSetup) {
    setup.quit<-"quit"
    setup.quit<-"quit"
    <-setup.ready
    <-setup.ready
}

func init() {
}

