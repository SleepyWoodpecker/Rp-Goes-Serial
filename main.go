package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"time"

	"go.bug.st/serial"
)

var serial_port = "/dev/cu.usbserial-10"

type MyStruct struct {
	One 		uint32
	Two 		uint32
	Sheesh [8]float32
}

func main() {
	messageQueue := make(chan [40]byte, 20)

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
	process(messageQueue)
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
		var one uint32;
		err := binary.Read(bytes.NewReader(tempBuff[:4]), binary.LittleEndian, &one)

		if err != nil {
			log.Fatal(err)
		}

		if prev >= 0 && prev != int(one) - 1 {
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

func process(messageQueue <-chan [40]byte) {
	file, err := os.OpenFile("output.txt", os.O_WRONLY | os.O_CREATE, 0644)
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

		message := fmt.Sprintf(
			"%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%d,%d",
			decodedStruct.Sheesh[0],
			decodedStruct.Sheesh[1],
			decodedStruct.Sheesh[2],
			decodedStruct.Sheesh[3],
			decodedStruct.Sheesh[4],
			decodedStruct.Sheesh[5],
			decodedStruct.Sheesh[6],
			decodedStruct.Sheesh[7],
			decodedStruct.One,
			decodedStruct.Two,
		)
		fmt.Printf("%s\n", message)
		fmt.Fprintf(writer, "%s\n", message)
	}
}