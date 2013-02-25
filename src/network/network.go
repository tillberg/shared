
package network;

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
  "../sharedpb"
  "../blob"
  "../types"
)

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

func SendMessage(message *sharedpb.Message, writer *bufio.Writer) error {
  return SendSignedMessage(message, writer, "")
}

// This is not a public-key cryptographic signature.  We should switch over
// to a proper signature when we start to deal with multi-user schemes.
func GenerateSignature(bytes []byte, key string) []byte  {
  h := sha256.New()
  h.Write([]byte(key))
  h.Write(bytes)
  h.Write([]byte(key))
  return h.Sum([]byte{})
}

func SendSignedMessage(message *sharedpb.Message, writer *bufio.Writer, apikey string) error {
  now := uint64(time.Now().Unix())
  message.Timestamp = &now
  messageBytes, err := proto.Marshal(message)
  if err != nil { return err }
  numMessageBytes := uint64(len(messageBytes))
  preamble := &sharedpb.Preamble{Length: &numMessageBytes}
  if apikey != "" {
    preamble.Signature = GenerateSignature(messageBytes, apikey)
  }
  preambleBytes, err := proto.Marshal(preamble)
  if err != nil { return err }
  WriteUvarint(writer, uint64(len(preambleBytes)))
  _, err = writer.Write(preambleBytes)
  if err != nil { return err }
  _, err = writer.Write(messageBytes)
  if err != nil { return err }
  writer.Flush()
  return nil
}

func ReceiveMessage(reader *bufio.Reader, apikey string) (*sharedpb.Message, bool, error) {
  preambleSize, err := binary.ReadUvarint(reader)
  if err != nil { return nil, false, err }
  bufPreamble := make([]byte, preambleSize)
  _, err = io.ReadFull(reader, bufPreamble)
  if err != nil { return nil, false, err }
  preamble := &sharedpb.Preamble{}
  err = proto.Unmarshal(bufPreamble, preamble)
  if err != nil { return nil, false, err }

  bufMessage := make([]byte, *preamble.Length)
  _, err = io.ReadFull(reader, bufMessage)
  if err != nil { return nil, false, err }
  message := &sharedpb.Message{}
  err = proto.Unmarshal(bufMessage, message)
  if err != nil { return nil, false, err }

  valid := false
  if preamble.Signature != nil {
    correct := GenerateSignature(bufMessage, apikey)
    valid = bytes.Equal(preamble.Signature, correct)
  }

  return message, valid, nil
}

func connOutgoing(conn *net.TCPConn, outbox chan *sharedpb.Message, apikey string) {
  s := "master"
  outbox<-&sharedpb.Message{SubscribeBranch: &s}
  types.BlobServicerChannel <- outbox
  writer := bufio.NewWriter(conn)
  for {
    message := <- outbox
    err := SendSignedMessage(message, writer, apikey)
    check(err)
    log.Printf("Sent %s", message.MessageString())
  }
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

func connIncoming(conn *net.TCPConn, outbox chan *sharedpb.Message, apikey string) {
  reader := bufio.NewReader(conn)
  for {
    message, valid, err := ReceiveMessage(reader, apikey)
    check(err)
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
    } else {
      log.Fatal("Unknown incoming message", message.MessageString())
    }
  }
}

func startConnections(conn *net.TCPConn, apikey string) {
  outbox := make(chan *sharedpb.Message, 10)
  go connOutgoing(conn, outbox, apikey)
  connIncoming(conn, outbox, apikey)
}

func makeConnection(remoteAddr *net.TCPAddr, apikey string) {
  start := time.Now()
  for {
    conn, err := net.DialTCP("tcp", nil, remoteAddr)
    if err != nil {
      if time.Since(start) > time.Second {
        log.Fatal(err)
      }
      time.Sleep(10 * time.Millisecond)
      continue
    }
    log.Printf("Connected to %s.", remoteAddr)
    startConnections(conn, apikey)
  }
}

func handleConnection(conn *net.TCPConn, apikey string) {
  log.Printf("Connection received from %s", conn.RemoteAddr().String())
  startConnections(conn, apikey)
}

func ListenForConnections(ln *net.TCPListener, apikey string) {
  for {
    conn, err := ln.AcceptTCP()
    if err != nil {
      log.Print(err)
      continue
    }
    go handleConnection(conn, apikey)
  }
}

func Start(listenPort int, apikey string) {
  listen_addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", listenPort));
  check(err)
  ln, err := net.ListenTCP("tcp", listen_addr)
  check(err)
  defer ln.Close()
  log.Printf("Listening on port %d.", listenPort)
  // XXX omg kludge.  Need to figure out how to properly negotiate
  // unique full-duplex P2P connections.
  if listenPort == 9252 {
    remote_addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:9251");
    check(err)
    go makeConnection(remote_addr, apikey)
  }
  ListenForConnections(ln, apikey)
}
