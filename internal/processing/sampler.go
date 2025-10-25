package processing

import (
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"
)

const SamplingChannelName = "pressurevals"

type sampler struct {
	samplingFrequency 	time.Duration
	udpConn 						*net.UDPConn
	storesToSampleFrom	[](*DataSampleStore)
	logger 							*zap.Logger
}

func NewSampler(samplingFrequency time.Duration, udpConn *net.UDPConn, stores [](*DataSampleStore), logger *zap.Logger) *sampler {
	return &sampler{
		samplingFrequency: samplingFrequency,
		udpConn: udpConn,
		storesToSampleFrom: stores,
		logger: logger,
	}
}

/* HV ports come before LV ports */
func (s *sampler) SampleAndLog(sampleBuffer []float32) {
	for idx, store := range s.storesToSampleFrom {
		writeStartIdx := idx * NumReadingsPerPacket
		lastSampleFromStore := store.GetReadingFromSampleStore()
		copy(sampleBuffer[writeStartIdx : writeStartIdx + NumReadingsPerPacket], lastSampleFromStore[:])
	}

	// format the string as an influx string to send to the grafana connection
	influxString := SamplingChannelName + " "
	for idx, ptReading := range sampleBuffer {
		influxString += fmt.Sprintf("pt%d=%.2f", idx, ptReading)
	}
	influxString += fmt.Sprintf(" %d", time.Now().UnixNano())

	err := s.sendToUDPConn(&influxString)
	if err != nil {
		s.logger.Warn("[sampler] Error writing data to UDP connection", zap.Error(err))
	} else {
		s.logger.Info("[sampler] collected sample", zap.String("influxString", influxString))
	}
}

func (s *sampler) Run() {
	sampleBuffer := make([]float32, len(s.storesToSampleFrom) * NumReadingsPerPacket)

	for range time.Tick(s.samplingFrequency) {
		s.SampleAndLog(sampleBuffer)
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