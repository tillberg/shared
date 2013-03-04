
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
  "../serializer"
  "../sharedpb"
  "../storage"
  "../types"
)

var apikey = ""

func GetShortHexString(bytes []byte) string {
  return GetHexString(bytes[:4])
}

func GetHexString(bytes []byte) string {
  return fmt.Sprintf("%#x", bytes)
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

func SendObject(hash types.Hash, dest chan *sharedpb.Message) {
  blob := blob.GetBlob(hash)
  data, err := serializer.Configured().Marshal(blob)
  if err != nil { log.Fatal(err) }
  compressed := storage.Configured().Deflate(data)
  // log.Printf("bytes: %d", len(bytes))
  dest <- &sharedpb.Message{Object: &sharedpb.Object{Hash: hash, Object: compressed}}
}

func SendSignedMessage(message *sharedpb.Message, writer *bufio.Writer) error {
  now := uint64(time.Now().Unix())
  message.Timestamp = &now
  // log.Printf("Going to send %s", message.MessageString())
  messageBytes, err := proto.Marshal(message)
  if err != nil { return err }
  numMessageBytes := uint64(len(messageBytes))
  preamble := &sharedpb.Preamble{Length: &numMessageBytes}
  preamble.Signature = GenerateSignature(messageBytes)
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

func SendSingleMessage(message *sharedpb.Message, address string) {
  start := time.Now()
  for {
    remoteAddr, err := net.ResolveTCPAddr("tcp", address)
    if err != nil { log.Fatal(err) }
    conn, err := net.DialTCP("tcp", nil, remoteAddr)
    if err != nil {
      if time.Since(start) > time.Second {
        log.Fatal(err)
      }
      time.Sleep(10 * time.Millisecond)
      continue
    }
    writer := bufio.NewWriter(conn)
    err = SendSignedMessage(message, writer)
    if err != nil {
      log.Printf("Error sending single message: %s", err)
    }
    break
  }
}

func ReceiveMessage(reader *bufio.Reader) (*sharedpb.Message, bool) {
  preambleSize, err := binary.ReadUvarint(reader)
  if err != nil { log.Println(err); return nil, false }
  bufPreamble := make([]byte, preambleSize)
  _, err = io.ReadFull(reader, bufPreamble)
  if err != nil { log.Println(err); return nil, false }
  preamble := &sharedpb.Preamble{}
  err = proto.Unmarshal(bufPreamble, preamble)
  if err != nil { log.Println(err); return nil, false }

  bufMessage := make([]byte, *preamble.Length)
  _, err = io.ReadFull(reader, bufMessage)
  if err != nil { log.Println(err); return nil, false }
  message := &sharedpb.Message{}
  err = proto.Unmarshal(bufMessage, message)
  if err != nil { log.Println(err); return nil, false }

  if preamble.Signature != nil {
    correct := GenerateSignature(bufMessage)
    if !bytes.Equal(preamble.Signature, correct) {
      log.Println("Invalid message received")
      return nil, false
    }
  }

  return message, true
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
    err := SendSignedMessage(message, writer)
    if err != nil {
      log.Printf("Error sending message: %s", err)
      break
    }
    // log.Printf("Sent %s", message.MessageString())
  }
}

func connIncoming(conn *net.TCPConn, outbox chan *sharedpb.Message) {
  reader := bufio.NewReader(conn)
  for {
    message, valid := ReceiveMessage(reader)
    if !valid { return }
    // log.Printf("Received %s", message.MessageString())
    if message.HashRequest != nil {
      go SendObject(message.HashRequest, outbox)
    } else if message.Object != nil {
      data, err := storage.Configured().Inflate(message.Object.Object)
      if err != nil { log.Fatal(err) }
      blob, err := serializer.Configured().Unmarshal(data)
      if err != nil { log.Fatal(err) }
      types.BlobReceiveChannel <- blob
    } else if message.Branch != nil {
      branchUpdate := types.BranchStatus{
        Name: fmt.Sprintf("origin/%s", *message.Branch.Name),
        Hash: message.Branch.Hash,
      }
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
    if err != nil { log.Fatal(err) }
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
  if err != nil { log.Fatal(err) }
  ln, err := net.ListenTCP("tcp", listen_addr)
  if err != nil { log.Fatal(err) }
  defer ln.Close()
  log.Printf("Listening on port %d.", listenPort)
  // XXX omg kludge.  Need to figure out how to properly negotiate
  // unique full-duplex P2P connections.
  ListenForConnections(ln)
}

func init() {
  config, err := conf.ReadConfigFile("shared.ini")
  if err != nil { log.Fatal(err) }
  apikey, err = config.GetString("main", "apikey")
  if err != nil { log.Fatal(err) }
}
