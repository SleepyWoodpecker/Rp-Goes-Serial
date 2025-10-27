package processing

import (
	"sync"
)

// The other way I thought sampling could have been done was by adding a timer in the processor that sent the data
// every 1/10 seconds to the sampler
// I think that might have been more performant, but it might also cause the samples to potentially be stale
// since the clocks on the two separate goroutines are not in sync

const NumReadingsPerPacket = 8

type DataSampleStore struct {
	RawReadings 		[NumReadingsPerPacket]float32
	BoardTimestamp 	int
	rawReadingMutex	sync.Mutex
}

func (d *DataSampleStore) UpdateSampleStore(newData []float32, boardTimeStamp int) {
	d.rawReadingMutex.Lock()
	defer d.rawReadingMutex.Unlock()

	copy(d.RawReadings[:], newData)
	d.BoardTimestamp = boardTimeStamp
}

func (d *DataSampleStore) GetReadingFromSampleStore() ([8]float32, int) {
	d.rawReadingMutex.Lock()
	defer d.rawReadingMutex.Unlock()

	return d.RawReadings, d.BoardTimestamp
}