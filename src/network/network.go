
package network;

import (
  "encoding/binary"
  "bufio"
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