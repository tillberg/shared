
package test

import (
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

func Launch(id string, cachePath string, syncPath string, port string, setup TestSetup) {
    cmd := exec.Cmd{
        Path: "shared",
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
    cmd.Process.Kill()
    cmd.Wait()
    setup.ready<-"exited"
}

type TestSetup struct {
    ready chan string
    quit chan string
}

func SetUp() *TestSetup {
    exec.Command("/usr/bin/go", "build", "src/shared.go")
    exec.Command("rm", "-rf", "/tmp/sync1").Run()
    exec.Command("mkdir", "/tmp/sync1").Run()
    exec.Command("rm", "-rf", "/tmp/sync2").Run()
    exec.Command("mkdir", "/tmp/sync2").Run()

    setup := TestSetup{ready: make(chan string), quit: make(chan string)}
    go Launch("A", "/tmp/cache1", "/tmp/sync1", "9251", setup)
    <-setup.ready
    go Launch("B", "/tmp/cache2", "/tmp/sync2", "9252", setup)
    <-setup.ready
    return &setup
}


func TearDown(setup *TestSetup) {
    setup.quit<-"quit"
    setup.quit<-"quit"
    <-setup.ready
    <-setup.ready
}

func init() {
}

