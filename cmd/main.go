package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"sleepywoodpecker/rp-goes-serial/internal/logger"
	"sleepywoodpecker/rp-goes-serial/internal/processing"
	rserial "sleepywoodpecker/rp-goes-serial/internal/rSerial"
	"syscall"
	"time"
)

const LOG_FILE_PATH = "rserial.logs"
const TELEGRAF_ADDR = "127.0.0.1:4020"
const PORT_HV = "/dev/serial/by-id/usb-Espressif_USB_JTAG_serial_debug_unit_B4:3A:45:B3:70:B0-if00"
const PORT_LV = "/dev/serial/by-id/usb-Espressif_USB_JTAG_serial_debug_unit_B4:3A:45:B6:7E:D0-if00"

const HV_CAL_LOG_FILE = "hv_cal.csv"
const HV_RAW_LOG_FILE = "hv_raw.csv"
const LV_CAL_LOG_FILE = "lv_cal.csv"
const LV_RAW_LOG_FILE = "lv_raw.csv"

const BAUDRATE = 460800

const MESSAGE_QUEUE_LENGTH = 20
var STOP_SEQUENCE = []byte{'\r', '\n'}

func main() {
	// context handler for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// first initialize the main logger
	logger, err := logger.NewLogger(LOG_FILE_PATH)
	if err != nil {	
		panic(err)
	}

	// initialize UDP connection to grafana
	udpAddr, err := net.ResolveUDPAddr("udp", TELEGRAF_ADDR)
	if err != nil {
		panic(err)
	}
	
	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		panic(err)
	}
	defer udpConn.Close()

	// initialize the sample store
	sampleStore := make([](*processing.DataSampleStore), 2)
	for i := range sampleStore {
		sampleStore[i] = processing.NewDataSampleStore()
	}

	// initialize the serial connections
	hvMessageQueue := make(chan []byte, MESSAGE_QUEUE_LENGTH)
	hvSerial := rserial.NewRSerial(PORT_HV, BAUDRATE, hvMessageQueue, logger, int(processing.PacketSize), STOP_SEQUENCE)
	defer hvSerial.Close()
	hvProcessor := processing.NewProcessor(HV_RAW_LOG_FILE, hvMessageQueue, logger, sampleStore[0])

	lvMessageQueue := make(chan []byte, MESSAGE_QUEUE_LENGTH)
	lvSerial := rserial.NewRSerial(PORT_LV, BAUDRATE, lvMessageQueue, logger, int(processing.PacketSize), STOP_SEQUENCE)
	defer lvSerial.Close()
	lvProcessor := processing.NewProcessor(LV_RAW_LOG_FILE, lvMessageQueue, logger, sampleStore[1])

	sampler := processing.NewSampler(100 * time.Millisecond, udpConn, sampleStore, logger)

	// run everything
	go hvProcessor.Run(ctx)
	go lvProcessor.Run(ctx)
	go hvSerial.Run(ctx)
	go lvSerial.Run(ctx)
	
	go sampler.Run(ctx)

	<-sigCh
	cancel()

	time.Sleep(500 * time.Millisecond)
}