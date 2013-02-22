
package main

import (
    "os/exec"
    "log"
    "fmt"
    "io"
    "bufio"
    "io/ioutil"
    "time"
)

func ReadLines(id string, pipe io.Reader) {
    r := bufio.NewReader(pipe)
    line, err := r.ReadString('\n')
    for err == nil {
        fmt.Printf("%s: %s", id, line)
        line, err = r.ReadString('\n')
    }
}

func Launch(id string, cachePath string, syncPath string, port string, ready chan string, quit chan string) {
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
    ready<-"ready"
    <-quit
    cmd.Process.Kill()
    cmd.Wait()
    ready<-"exited"
}

func main() {
    exec.Command("/usr/bin/go", "build", "src/shared.go")
    exec.Command("rm", "-rf", "/tmp/sync1").Run()
    exec.Command("mkdir", "/tmp/sync1").Run()
    exec.Command("rm", "-rf", "/tmp/sync2").Run()
    exec.Command("mkdir", "/tmp/sync2").Run()

    ready := make(chan string)
    quit := make(chan string)
    go Launch("A", "/tmp/cache1", "/tmp/sync1", "9251", ready, quit)
    <-ready
    go Launch("B", "/tmp/cache2", "/tmp/sync2", "9252", ready, quit)
    <-ready
    time.Sleep(500 * time.Millisecond)
    ioutil.WriteFile("/tmp/sync1/testfile", []byte{}, 0644)
    time.Sleep(500 * time.Millisecond)
    quit<-"quit"
    quit<-"quit"
    <-ready
    <-ready
    _, err := ioutil.ReadFile("/tmp/sync2/testfile")
    if err != nil {
        log.Fatal(err)
    }
}
