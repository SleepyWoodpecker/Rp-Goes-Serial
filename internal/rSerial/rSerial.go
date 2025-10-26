// r in rserial stands for "robust"
package rserial

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sleepywoodpecker/rp-goes-serial/internal/processing"
	"time"

	"go.bug.st/serial"
	"go.uber.org/zap"
)

type rserial struct {
	serial.Port
	MessageQueue 				chan<- []byte 		// channels are all implicitly passed as pointers
	tempBuff						[]byte
	logger							*zap.Logger
	portName						string
	stopSequence				[]byte
	rawPacketSize				int
}

type OutOfSyncError struct {
	ByteSequence 	[]byte
}

func (e *OutOfSyncError) Error() string {
	return fmt.Sprintf("Incorrect stop sequence detected %v", e.ByteSequence)
}

func NewRSerial(portName string, baudrate int, messageQueue chan<- []byte, logger *zap.Logger, rawPacketSize int, stopSequence []byte) *rserial {
	mode := &serial.Mode{
		BaudRate: baudrate,
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		logger.Fatal("Error opening serial port", zap.Error(err), zap.String("portName", portName))
	}

	return &rserial{
		Port: port,
		MessageQueue: messageQueue,
		tempBuff: make([]byte, rawPacketSize),
		logger: logger,
		portName: portName,
		stopSequence: stopSequence,
		rawPacketSize: rawPacketSize,
	}
}

func (r *rserial) initialize() {
	r.SetReadTimeout(time.Duration(5 * float64(time.Millisecond)))
	r.ResetInputBuffer()
	r.sync()
}

// this is for the 
func (r *rserial) Run(ctx context.Context) {
	// sync the serial port
	r.initialize()

	for {
		select {
		case <- ctx.Done():
			r.logger.Info("[rserial] exiting from rserial read loop", zap.String("portName", r.portName))
			close(r.MessageQueue)
			return
		default:
			err := r.ReadPacket()
			if err != nil {
				var oosError *OutOfSyncError
				if errors.As(err, &oosError) {
					r.logger.Warn("Error while attempting to read packet from serial", zap.Error(err) ,zap.String("portName", r.portName), zap.ByteString("payload", oosError.ByteSequence))
				} else {
					r.logger.Warn("Error while attempting to read packet from serial", zap.Error(err) ,zap.String("portName", r.portName))
				}

				r.sync()
			}
		}
	}
}

func (r *rserial) ReadPacket() error {
	count := 0
	for count < r.rawPacketSize {
		// TODO: find out what happens if this: (1) reads < 42 first (2) goes beyond the stop sequence into the next packet
		// go out of sync?
		n, err := r.Read(r.tempBuff[count:])
		if err != nil {
			return err
		}
		count += n
	}

	// copy array rather than sending r.tempBuff which would point to the underlying array all the time
	// this may put some more load on the gc but that is okay for now
	byteSequenceCopy := make([]byte, r.rawPacketSize)
	copy(byteSequenceCopy[:], r.tempBuff[:])

	// validate that the packet is valid by checking the last 2 characters of the packet
	if !bytes.Equal(r.tempBuff[r.rawPacketSize - 2:], r.stopSequence) {
		return &OutOfSyncError{
			ByteSequence: byteSequenceCopy,
		}
	}

	r.MessageQueue <- byteSequenceCopy
	return nil
}

func (r *rserial) sync() {
	r.logger.Warn("Resyncing serial port", zap.String("portName", r.portName))
	twoBytes := [2]byte{ 0x0, 0x0 }
	oneByte := [1]byte{}

	for !bytes.Equal(twoBytes[:], processing.StopSequence[:]) {
		_, err := r.Read(oneByte[:])
		if err != nil {
			r.logger.Warn("Error while resyncing serial port", zap.Error(err) ,zap.String("portName", r.portName))
		}

		// update the two byte sequence
		twoBytes[0] = twoBytes[1]
		twoBytes[1] = oneByte[0]
	}
}