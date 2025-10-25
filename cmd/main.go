package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"sleepywoodpecker/rp-goes-serial/internal/logger"
	"time"

	"go.bug.st/serial"
)

var serial_port_lv = "/dev/serial/by-id/usb-Espressif_USB_JTAG_serial_debug_unit_B4:3A:45:B6:7E:D0-if00"
var serial_port_hv = "/dev/serial/by-id/usb-Espressif_USB_JTAG_serial_debug_unit_B4:3A:45:B3:70:B0-if00"

var lv_filename = "lv.csv"
var hv_filename = "hv.csv"
var warnLogFilename = "warnlogs.log"

type MyStruct struct {
	One    uint32
	Two    uint32
	Sheesh [8]float32
}

func main() {
	// instantiate the logger
	logger, err := logger.NewLogger(warnLogFilename)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	lv_message_queue := make(chan [40]byte, 20)
	hv_message_queue := make(chan [40]byte, 20)

	mode := &serial.Mode{
		BaudRate: 460800,
	}
	lv_port, err := serial.Open(serial_port_lv, mode)
	if err != nil {
		log.Fatal(err)
	}
	defer lv_port.Close()

	hv_port, err := serial.Open(serial_port_hv, mode)
	if err != nil {
		log.Fatal(err)
	}
	defer hv_port.Close()

	lv_port.SetReadTimeout(time.Duration(5 * float64(time.Millisecond)))
	lv_port.ResetInputBuffer()

	hv_port.SetReadTimeout(time.Duration(5 * float64(time.Millisecond)))
	lv_port.ResetInputBuffer()

	reSync(lv_port)
	reSync(hv_port)

	go read(lv_message_queue, lv_port)
	go process(lv_message_queue, lv_filename, true)
	go read(hv_message_queue, hv_port)
	process(hv_message_queue, hv_filename, false)
}

func read(messageQueue chan<- [40]byte, port serial.Port) {
	prev := -1
	tempBuff := make([]byte, 42)

	for {
		count := 0
		for count < 42 {
			n, err := port.Read(tempBuff[count:])
			if err != nil {
				log.Fatal(err)
			}
			count += n
		}

		// try checking the first int
		var one uint32
		err := binary.Read(bytes.NewReader(tempBuff[:4]), binary.LittleEndian, &one)

		if err != nil {
			log.Fatal(err)
		}

		if prev >= 0 && prev != int(one)-1 {
			reSync(port)
			prev = -1
		} else {
			prev = int(one)
		}

		messageQueue <- [40]byte(tempBuff)
	}
}

func reSync(port serial.Port) {
	fmt.Println("Resyncing")
	onebyte := make([]byte, 1)

	for onebyte[0] != '\n' {
		_, err := port.Read(onebyte)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func process(messageQueue <-chan [40]byte, filename string, isLv bool) {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	writer := bufio.NewWriter(file)

	for tempBuff := range messageQueue {
		var decodedStruct MyStruct
		err := binary.Read(bytes.NewReader(tempBuff[:40]), binary.LittleEndian, &decodedStruct)

		if err != nil {
			log.Fatal(err)
		}

		var message string
		if isLv {
			message = fmt.Sprintf(
				"%d,%d,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f",
				decodedStruct.One,
				decodedStruct.Two,
				decodedStruct.Sheesh[0],
				decodedStruct.Sheesh[1],
				decodedStruct.Sheesh[2],
				decodedStruct.Sheesh[3],
				decodedStruct.Sheesh[4],
				decodedStruct.Sheesh[5],
				decodedStruct.Sheesh[6],
				decodedStruct.Sheesh[7],
				
			)
			fmt.Printf("LV: %s\n", message)
		} else {
			message = fmt.Sprintf(
				"%d,%d,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f",
				decodedStruct.One,
				decodedStruct.Two,
				decodedStruct.Sheesh[0],
				decodedStruct.Sheesh[1],
				decodedStruct.Sheesh[2],
				decodedStruct.Sheesh[3],
				decodedStruct.Sheesh[4],
				decodedStruct.Sheesh[5],
				decodedStruct.Sheesh[6],
				decodedStruct.Sheesh[7],
			)
			fmt.Printf("HV: %s\n", message)
		}
		fmt.Fprintf(writer, "%s\n", message)
	}
}
