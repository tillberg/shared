
package network;

import (
  "encoding/binary"
  "bufio"
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
  now := uint64(time.Now().Unix())
  message.Timestamp = &now
  messageBytes, err := proto.Marshal(message)
  if err != nil { return err }
  numMessageBytes := uint64(len(messageBytes))
  preamble := &sharedpb.Preamble{Length: &numMessageBytes}
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

func ReceiveMessage(reader *bufio.Reader) (*sharedpb.Preamble, *sharedpb.Message, error) {
  preambleSize, err := binary.ReadUvarint(reader)
  if err != nil { return nil, nil, err }
  bufPreamble := make([]byte, preambleSize)
  _, err = io.ReadFull(reader, bufPreamble)
  if err != nil { return nil, nil, err }
  preamble := &sharedpb.Preamble{}
  err = proto.Unmarshal(bufPreamble, preamble)
  if err != nil { return nil, nil, err }

  bufMessage := make([]byte, *preamble.Length)
  _, err = io.ReadFull(reader, bufMessage)
  if err != nil { return nil, nil, err }
  message := &sharedpb.Message{}
  err = proto.Unmarshal(bufMessage, message)
  if err != nil { return nil, nil, err }

  return preamble, message, nil
}