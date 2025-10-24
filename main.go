package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"go.bug.st/serial"
)

var serial_port = "/dev/cu.usbserial-110"

type MyStruct struct {
	Sheesh [32]byte
	One 		uint32
	Two uint32
}

func main() {
	messageQueue := make(chan [40]byte, 20)
	resetChannel := make(struct{})

	mode := &serial.Mode{
		BaudRate: 460800,
	}
	port, err := serial.Open(serial_port, mode)
	if err != nil {
		log.Fatal(err)
	}
	defer port.Close()

	port.SetReadTimeout(time.Duration(5 * float64(time.Millisecond)))
	port.ResetInputBuffer()

	reSync(port)

	go read(messageQueue, port)
	process(messageQueue, port)
}

func read(messageQueue chan<- [40]byte, resetChannel chan<- struct{}, port serial.Port) {
	tempBuff := make([]byte, 42)
	for {
		select {
		case <-resetChannel:
		}

		count := 0
		for count < 42 {
			n, err := port.Read(tempBuff[count:])
			if err != nil {
				log.Fatal(err)
			}
			count += n
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

func process(messageQueue <-chan [40]byte, port serial.Port) {
	prev := -1
	for tempBuff := range messageQueue {
		var decodedStruct MyStruct
		err := binary.Read(bytes.NewReader(tempBuff[:40]), binary.LittleEndian, &decodedStruct)

		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("%v\n", decodedStruct)

		if prev >= 0 && prev != int(decodedStruct.One) - 1 {
			reSync(port)
			prev = -1
		} else {
			prev = int(decodedStruct.One)
		}
	}
}