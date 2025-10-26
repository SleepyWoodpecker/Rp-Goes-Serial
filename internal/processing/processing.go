package processing

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"unsafe"

	"go.uber.org/zap"
)

const DEFAULT_QUEUE_SIZE = 20

type Processor struct {
	Filename 			string
	MessageQueue 	<-chan [40]byte
	logger				*zap.Logger
}

type DataPacket struct {
	PacketNumber  uint32
	Timestamp    	uint32
	RawReadings 	[8]float32
}

const PacketSize = unsafe.Sizeof(DataPacket{})
var StopSequence = [2]byte{'\r', '\n'}

func NewProcessor(filename string, messageQueue <-chan [40]byte, logger *zap.Logger) (*Processor) {
	return &Processor{
		Filename: 		filename,
		MessageQueue: messageQueue,
		logger: 			logger,	
	}
}

func (p *Processor) Run() error {
	file, err := os.OpenFile(p.Filename, os.O_WRONLY | os.O_CREATE, 0644)

	if err != nil {
		p.logger.Fatal("Error opening a file", zap.Error(err), zap.String("outputFile", p.Filename))
	}

	writer := bufio.NewWriter(file)

	for packet := range p.MessageQueue {
		err := p.ProcessPacket(packet[:], writer) 
		if err != nil {
			p.logger.Warn(
				"Error decoding byte packet", 
				zap.Error(err),
				zap.Int("packetLength", len(packet)),
				zap.String("outputFile", p.Filename),
				zap.ByteString("rawBytes", packet[:]),
			)
			continue
		}
	}

	return nil
}

func (p *Processor) ProcessPacket(packet []byte, outStream io.Writer) error {
	var decodedStruct DataPacket
	err := binary.Read(bytes.NewReader(packet[:40]), binary.LittleEndian, &decodedStruct)
	if err != nil {
		// TODO: test this error by creating a decode error
		return err
	}

	// create string representation of data
	message := fmt.Sprintf(
		"%d,%d,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f",
		decodedStruct.PacketNumber,
		decodedStruct.Timestamp,
		decodedStruct.RawReadings[0],
		decodedStruct.RawReadings[1],
		decodedStruct.RawReadings[2],
		decodedStruct.RawReadings[3],
		decodedStruct.RawReadings[4],
		decodedStruct.RawReadings[5],
		decodedStruct.RawReadings[6],
		decodedStruct.RawReadings[7],
	)

	fmt.Fprint(outStream, message)
	return nil
}