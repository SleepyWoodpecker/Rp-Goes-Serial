package processing

import (
	"bufio"
	"bytes"
	"context"
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
	MessageQueue 	<-chan []byte
	logger				*zap.Logger
	dataStore 		*DataSampleStore
}

type DataPacket struct {
	PacketNumber  uint32
	Timestamp    	uint32
	RawReadings 	[8]float32
}

const PacketSize = unsafe.Sizeof(DataPacket{})
var StopSequence = [2]byte{'\r', '\n'}

func NewProcessor(filename string, messageQueue <-chan []byte, logger *zap.Logger, dataStore *DataSampleStore) (*Processor) {
	return &Processor{
		Filename: 		filename,
		MessageQueue: messageQueue,
		logger: 			logger,	
		dataStore: 		dataStore,	
	}
}

func (p *Processor) Run(ctx context.Context) error {
	file, err := os.OpenFile(p.Filename, os.O_WRONLY | os.O_CREATE, 0644)
	if err != nil {
		p.logger.Fatal("[processor] error opening a file", zap.Error(err), zap.String("outputFile", p.Filename))
	}
	defer file.Close()


	writer := bufio.NewWriter(file)
	defer writer.Flush()

	for {
		select {
		case packet, ok := <-p.MessageQueue:
			if !ok {
				p.logger.Info("[processor] message queue closed", zap.String("outputFile", p.Filename))
				return nil
			}

			if err := p.ProcessPacket(packet[:], writer); err != nil {
				p.logger.Warn(
					"[processor] error decoding byte packet", 
					zap.Error(err),
					zap.Int("packetLength", len(packet)),
					zap.String("outputFile", p.Filename),
					zap.ByteString("rawBytes", packet[:]),
				)
			}
		case <-ctx.Done():
			p.logger.Info("[processor] received shutdown signal", zap.String("outputFile", p.Filename))
			return nil
		}
	}
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
		"%d,%d,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f\n",
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
	p.dataStore.UpdateSampleStore(decodedStruct.RawReadings[:])

	return nil
}