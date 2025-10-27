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
const NumPtsPerBoard = 8

type Processor struct {
	RawFileName 	string
	CalFileName		string
	MessageQueue 	<-chan []byte
	logger				*zap.Logger
	dataStore 		*DataSampleStore
	ACoeffs				[NumPtsPerBoard]float32
	BCoeffs				[NumPtsPerBoard]float32
}

type DataPacket struct {
	PacketNumber  uint32
	Timestamp    	uint32
	RawReadings 	[NumPtsPerBoard]float32
}

const PacketSize = unsafe.Sizeof(DataPacket{})
var StopSequence = [2]byte{'\r', '\n'}

func NewProcessor(
	rawFilename string, 
	calFilename string,
	messageQueue <-chan []byte, 
	logger *zap.Logger, 
	dataStore *DataSampleStore, 
	aCoeffs [NumPtsPerBoard]float32, 
	bCoeffs [NumPtsPerBoard]float32,
) (*Processor) {
	return &Processor{
		RawFileName: 	rawFilename,
		CalFileName: 	calFilename,
		MessageQueue: messageQueue,
		logger: 			logger,	
		dataStore: 		dataStore,	
		ACoeffs: 			aCoeffs,	
		BCoeffs: 			bCoeffs,
	}
}

func (p *Processor) Run(ctx context.Context) error {
	rawFile, err := os.OpenFile(p.RawFileName, os.O_WRONLY | os.O_CREATE, 0644)
	if err != nil {
		p.logger.Fatal("[processor] error opening a file", zap.Error(err), zap.String("outputFile", p.RawFileName))
	}
	defer rawFile.Close()
	rawFileWriter := bufio.NewWriter(rawFile)
	defer rawFileWriter.Flush()

	calFile, err := os.OpenFile(p.CalFileName, os.O_WRONLY | os.O_CREATE, 0644)
	if err != nil {
		p.logger.Fatal("[processor] error opening a file", zap.Error(err), zap.String("outputFile", p.CalFileName))
	}
	defer calFile.Close()
	calFileWriter := bufio.NewWriter(calFile)
	defer calFileWriter.Flush()

	for {
		select {
		case packet, ok := <-p.MessageQueue:
			if !ok {
				p.logger.Info("[processor] message queue closed", zap.String("rawFileName", p.RawFileName))
				return nil
			}

			if err := p.ProcessPacket(packet[:], rawFileWriter, calFileWriter); err != nil {
				p.logger.Warn(
					"[processor] error decoding byte packet", 
					zap.Error(err),
					zap.Int("packetLength", len(packet)),
					zap.String("rawFileName", p.RawFileName),
					zap.ByteString("rawBytes", packet[:]),
				)
			}
		case <-ctx.Done():
			p.logger.Info("[processor] received shutdown signal", zap.String("rawFileName", p.RawFileName))
			return nil
		}
	}
}

func (p *Processor) ProcessPacket(packet []byte, rawFileStream io.Writer, calFileStream io.Writer) error {
	var decodedStruct DataPacket
	err := binary.Read(bytes.NewReader(packet[:40]), binary.LittleEndian, &decodedStruct)
	if err != nil {
		// TODO: test this error by creating a decode error
		return err
	}

	// create string representation of data
	fmt.Fprintf(
		rawFileStream, 
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

	for i := range decodedStruct.RawReadings {
		decodedStruct.RawReadings[i] = decodedStruct.RawReadings[i] * p.ACoeffs[i] + p.BCoeffs[i]
	}

	fmt.Fprintf(
		calFileStream,
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

	// send the calibrated readings to the sample store
	p.dataStore.UpdateSampleStore(decodedStruct.RawReadings[:], int(decodedStruct.Timestamp))

	return nil
}