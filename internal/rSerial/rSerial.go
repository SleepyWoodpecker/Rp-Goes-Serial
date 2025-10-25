// r in rserial stands for "robust"
package rserial

import (
	"bytes"
	"time"

	"go.bug.st/serial"
	"go.uber.org/zap"
)

type rserial struct {
	serial.Port
	MessageQueue 				chan<- []byte 		// channels are all implicitly passed as pointers
	tempBuff						[]byte
	logger							*zap.Logger
	hasBeenInitialized	bool
	portName						string
	stopSequence				[]byte
	rawPacketSize				int
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

func (r *rserial) Initialize() {
	if r.hasBeenInitialized {
		r.logger.Fatal("rSerial object has already been initialized before", zap.String("portName", r.portName))
	}

	r.SetReadTimeout(time.Duration(5 * float64(time.Millisecond)))
	r.ResetInputBuffer()
	r.hasBeenInitialized = true
}

// this is for the 
func (r *rserial) ReadPacket() {
	if !r.hasBeenInitialized {
		r.logger.Fatal("rSerial object has not been initialized yet!", zap.String("portName", r.portName))
	}

	// sync the serial port
	r.sync()

	for {
		count := 0
		for count < r.rawPacketSize {
			// TODO: find out what happens if this: (1) reads < 42 first (2) goes beyond the stop sequence into the next packet
			// go out of sync?
			n, err := r.Read(r.tempBuff[count:])
			if err != nil {
				r.logger.Warn("Error while attempting to read packet from serial", zap.Error(err) ,zap.String("portName", r.portName))
				break
			}
			count += n
		}

		// validate that the packet is valid by checking the last 2 characters of the packet
		if len(r.tempBuff) != r.rawPacketSize || !bytes.Equal(r.tempBuff[r.rawPacketSize - 2:], r.stopSequence) {
			r.sync()
		}
	}
}

func (r *rserial) sync() {
	r.logger.Warn("Resyncing serial port", zap.String("portName", r.portName))
	onebyte := make([]byte, 1)

	for onebyte[0] != r.stopSequence[r.rawPacketSize - 1] {
		_, err := r.Read(onebyte)
		if err != nil {
			r.logger.Warn("Error while resyncing serial port", zap.Error(err) ,zap.String("portName", r.portName))
		}
	}
}