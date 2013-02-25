
package network;

import (
  "bufio"
  "bytes"
  "crypto/sha256"
  "encoding/binary"
  "io"
  "time"
  "code.google.com/p/goprotobuf/proto"
  "../sharedpb"
)

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
