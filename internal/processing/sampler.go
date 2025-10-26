package processing

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"
)

const SamplingChannelName = "pressurevals"

type sampler struct {
	samplingFrequency 		time.Duration
	udpConn 							*net.UDPConn
	storesToSampleFrom		[](*DataSampleStore)
	logger 								*zap.Logger
	firstBoardTimestamp		int										// this is the first timestamp received from the ESP32
	samplerInitTimestamp 	int64
}

func NewSampler(samplingFrequency time.Duration, udpConn *net.UDPConn, stores [](*DataSampleStore), logger *zap.Logger) *sampler {
	return &sampler{
		samplingFrequency: samplingFrequency,
		udpConn: udpConn,
		storesToSampleFrom: stores,
		logger: logger,
		firstBoardTimestamp: 0,
		samplerInitTimestamp: time.Now().UnixNano(),
	}
}

/* HV ports come before LV ports */
func (s *sampler) SampleAndLog(sampleBuffer []float32) {
	var lastBoardTimeStamp int
	var lastSampleFromStore [8]float32

	for idx, store := range s.storesToSampleFrom {
		writeStartIdx := idx * NumReadingsPerPacket
		lastSampleFromStore, lastBoardTimeStamp = store.GetReadingFromSampleStore()
		copy(sampleBuffer[writeStartIdx : writeStartIdx + NumReadingsPerPacket], lastSampleFromStore[:])
	}

	// set a base timestamp if there has not been one yet
	if s.firstBoardTimestamp == 0 {
		s.firstBoardTimestamp = lastBoardTimeStamp
	}

	// get the offset from the first reading
	timeSinceStartNs := int64(lastBoardTimeStamp - s.firstBoardTimestamp) * 1000000

	// format the string as an influx string to send to the grafana connection
	influxString := SamplingChannelName + " "
	for idx, ptReading := range sampleBuffer {
		influxString += fmt.Sprintf("pt%d=%.2f,", idx, ptReading)
	}
	influxString = influxString[:len(influxString) - 1]
	influxString += fmt.Sprintf(" %d", (s.samplerInitTimestamp + int64(timeSinceStartNs)))

	err := s.sendToUDPConn(&influxString)
	if err != nil {
		s.logger.Warn("[sampler] Error writing data to UDP connection", zap.Error(err), zap.String("influxString", influxString))
	}
}

func (s *sampler) Run(ctx context.Context) {
	sampleBuffer := make([]float32, len(s.storesToSampleFrom) * NumReadingsPerPacket)
	ticker := time.NewTicker(s.samplingFrequency)

	for {
		select {
		case <-ticker.C:
			s.SampleAndLog(sampleBuffer)
		case <-ctx.Done():
			return
		}
	}
}

func (s *sampler) sendToUDPConn(formattedData *string) error {
	totalWritten := 0
	for totalWritten < len(*formattedData) {
		n, err := s.udpConn.Write([]byte(*formattedData))
		if err != nil {
			return err
		}
		totalWritten += n
	}

	return nil
}