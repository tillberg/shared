
package network

import (
  "bufio"
  "bytes"
  "crypto/sha256"
  "encoding/binary"
  "fmt"
  "log"
  "io"
  "net"
  "time"
  "code.google.com/p/goprotobuf/proto"
  "github.com/tillberg/goconfig/conf"
  "../blob"
  "../sharedpb"
  "../types"
)

var apikey = ""

func GetShortHexString(bytes []byte) string {
  return GetHexString(bytes[:8])
}

func GetHexString(bytes []byte) string {
  return fmt.Sprintf("%#x", bytes)
}

func check(err interface{}) {
  if err != nil {
    log.Fatal(err)
  }
}

func WriteUvarint(writer *bufio.Writer, number uint64) {
  buf := make([]byte, 8)
  numBytes := binary.PutUvarint(buf, number)
  writer.Write(buf[:numBytes])
}

// This is not a public-key cryptographic signature.  We should switch over
// to a proper signature when we start to deal with multi-user schemes.
func GenerateSignature(bytes []byte) []byte  {
  h := sha256.New()
  h.Write([]byte(apikey))
  h.Write(bytes)
  h.Write([]byte(apikey))
  return h.Sum([]byte{})
}

func SendSignedMessage(message *sharedpb.Message, writer *bufio.Writer) {
  now := uint64(time.Now().Unix())
  message.Timestamp = &now
  messageBytes, err := proto.Marshal(message)
  check(err)
  numMessageBytes := uint64(len(messageBytes))
  preamble := &sharedpb.Preamble{Length: &numMessageBytes}
  preamble.Signature = GenerateSignature(messageBytes)
  preambleBytes, err := proto.Marshal(preamble)
  check(err)
  WriteUvarint(writer, uint64(len(preambleBytes)))
  _, err = writer.Write(preambleBytes)
  check(err)
  _, err = writer.Write(messageBytes)
  check(err)
  writer.Flush()
}

func SendSingleMessage(message *sharedpb.Message, address string) {
  start := time.Now()
  for {
    remoteAddr, err := net.ResolveTCPAddr("tcp", address)
    check(err)
    conn, err := net.DialTCP("tcp", nil, remoteAddr)
    if err != nil {
      if time.Since(start) > time.Second {
        log.Fatal(err)
      }
      time.Sleep(10 * time.Millisecond)
      continue
    }
    writer := bufio.NewWriter(conn)
    SendSignedMessage(message, writer)
    break
  }
}

func ReceiveMessage(reader *bufio.Reader) (*sharedpb.Message, bool) {
  preambleSize, err := binary.ReadUvarint(reader)
  check(err)
  bufPreamble := make([]byte, preambleSize)
  _, err = io.ReadFull(reader, bufPreamble)
  check(err)
  preamble := &sharedpb.Preamble{}
  err = proto.Unmarshal(bufPreamble, preamble)
  check(err)

  bufMessage := make([]byte, *preamble.Length)
  _, err = io.ReadFull(reader, bufMessage)
  check(err)
  message := &sharedpb.Message{}
  err = proto.Unmarshal(bufMessage, message)
  check(err)

  valid := false
  if preamble.Signature != nil {
    correct := GenerateSignature(bufMessage)
    valid = bytes.Equal(preamble.Signature, correct)
  }

  return message, valid
}

func SubscribeToBranch(name string, outbox chan *sharedpb.Message) {
  updateChannel := make(chan types.BranchStatus, 10)
  types.BranchSubscribeChannel <- types.BranchSubscription{Name:name, ResponseChannel: updateChannel}
  for {
    select {
      case update := <-updateChannel:
        outbox <- &sharedpb.Message{Branch: &sharedpb.Branch{Name: &name, Hash: update.Hash}}
    }
  }
}

func connOutgoing(conn *net.TCPConn, outbox chan *sharedpb.Message) {
  s := "master"
  outbox<-&sharedpb.Message{SubscribeBranch: &s}
  types.BlobServicerChannel <- outbox
  writer := bufio.NewWriter(conn)
  for {
    message := <- outbox
    SendSignedMessage(message, writer)
    log.Printf("Sent %s", message.MessageString())
  }
}

func connIncoming(conn *net.TCPConn, outbox chan *sharedpb.Message) {
  reader := bufio.NewReader(conn)
  for {
    message, valid := ReceiveMessage(reader)
    if !valid {
      log.Fatal("Invalid message received")
    }
    log.Printf("Received %s", message.MessageString())
    if message.HashRequest != nil {
      go blob.SendObject(message.HashRequest, outbox)
    } else if message.Object != nil {
      types.BlobReceiveChannel <- message.Object.Object
    } else if message.Branch != nil {
      branchUpdate := types.BranchStatus{Name: *message.Branch.Name, Hash:message.Branch.Hash}
      types.BranchUpdateChannel <- branchUpdate
    } else if message.SubscribeBranch != nil {
      go SubscribeToBranch(*message.SubscribeBranch, outbox)
    } else if message.AddRemote != nil {
      for _, address := range message.AddRemote {
        go makeConnection(address)
      }
    } else {
      log.Fatal("Unknown incoming message", message.MessageString())
    }
  }
}

func startConnections(conn *net.TCPConn) {
  outbox := make(chan *sharedpb.Message, 10)
  go connOutgoing(conn, outbox)
  connIncoming(conn, outbox)
}

func makeConnection(address string) {
  start := time.Now()
  for {
    remoteAddr, err := net.ResolveTCPAddr("tcp", address)
    check(err)
    conn, err := net.DialTCP("tcp", nil, remoteAddr)
    if err != nil {
      if time.Since(start) > time.Second {
        log.Fatal(err)
      }
      time.Sleep(10 * time.Millisecond)
      continue
    }
    log.Printf("Connected to %s.", address)
    startConnections(conn)
  }
}

func handleConnection(conn *net.TCPConn) {
  log.Printf("Connection received from %s", conn.RemoteAddr().String())
  startConnections(conn)
}

func ListenForConnections(ln *net.TCPListener) {
  for {
    conn, err := ln.AcceptTCP()
    if err != nil {
      log.Print(err)
      continue
    }
    go handleConnection(conn)
  }
}

func Start(listenPort int) {
  listen_addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", listenPort))
  check(err)
  ln, err := net.ListenTCP("tcp", listen_addr)
  check(err)
  defer ln.Close()
  log.Printf("Listening on port %d.", listenPort)
  // XXX omg kludge.  Need to figure out how to properly negotiate
  // unique full-duplex P2P connections.
  ListenForConnections(ln)
}

func init() {
  config, err := conf.ReadConfigFile("shared.ini")
  check(err)
  apikey, err = config.GetString("main", "apikey")
  check(err)
}
