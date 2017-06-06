package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	TCPv4Path = "/proc/net/tcp"
	TCPv6Path = "/proc/net/tcp6"
	UDPv4Path = "/proc/net/udp"
	UDPv6Path = "/proc/net/udp6"

	IPv4String = "IPv4"
	IPv6String = "IPv6"

	TCPv4Str = "TCPv4"
	TCPv6Str = "TCPv6"
	UDPv4Str = "UDPv4"
	UDPv6Str = "UDPv6"
)

var (
	GlobalTCPv4Records map[uint64]*GenericRecord
	GlobalTCPv6Records map[uint64]*GenericRecord
	GlobalUDPv4Records map[uint64]*GenericRecord
	GlobalUDPv6Records map[uint64]*GenericRecord

	Sstate = []string{
		"UNKNOWN",
		"ESTAB",
		"SYN-SENT",
		"SYN-RECV",
		"FIN-WAIT-1",
		"FIN-WAIT-2",
		"TIME-WAIT",
		"UNCONN",
		"CLOSE-WAIT",
		"LAST-ACK",
		"LISTEN",
		"CLOSING",
	}

	TimerName = []string{
		"OFF",
		"ON",
		"KEEPALIVE",
		"TIMEWAIT",
		"PERSIST",
		"UNKNOWN",
	}

	SstateActive = map[int]bool{
		0:  false,
		1:  true,
		2:  false,
		3:  false,
		4:  false,
		5:  false,
		6:  false,
		7:  false,
		8:  false,
		9:  false,
		10: true,
		11: false,
	}

	Colons = []string{
		":::::::",
		"::::::",
		":::::",
		"::::",
		":::",
	}
)

type IP struct {
	Host string
	Port string
}

func (i IP) String() (str string) {
	return i.Host + ":" + i.Port
}

func IPv4HexToString(ipHex string) (ip string, err error) {
	var tempInt int64
	if len(ipHex) != 8 {
		fmt.Printf("invalid input:[%s]\n", ipHex)
		return ip, fmt.Errorf("invalid input:[%s]", ipHex)
	}
	for i := 3; i > 0; i-- {
		if tempInt, err = strconv.ParseInt(ipHex[i*2:(i+1)*2], 16, 64); err != nil {
			fmt.Println(err)
			return "", err
		}
		ip += fmt.Sprintf("%d", tempInt) + "."
	}
	if tempInt, err = strconv.ParseInt(ipHex[0:2], 16, 64); err != nil {
		fmt.Println(err)
		return "", err
	}
	ip += fmt.Sprintf("%d", tempInt)
	return ip, nil
}

func IPv6HexToString(ipHex string) (ip string, err error) {
	prefix := ipHex[:24]
	suffix := ipHex[24:]
	for i := 0; i < 6; i++ {
		if prefix[i:i+4] == "0000" {
			ip += ":"
			continue
		}
		ip += prefix[i:i+4] + ":"
	}
	for _, v := range Colons {
		ip = strings.Replace(ip, v, "::", -1)
	}
	if suffix, err = IPv4HexToString(suffix); err != nil {
		fmt.Println(err)
		return "", err
	}
	ip += suffix
	return ip, nil
}

type GenericRecord struct {
	LocalAddr  IP
	RemoteAddr IP
	Status     int
	TxQueue    int
	RxQueue    int
	Timer      int
	Timeout    int
	Retransmit int
	UID        uint64
	Probes     int // unanswered 0-window probes
	Inode      uint64
	RefCount   int
	SK         uint64
	// TCP Specific
	RTO                int // RetransmitTimeout
	ATO                int // Predicted tick of soft clock (delayed ACK control data)
	QACK               int // (ack.quick<<1)|ack.pingpong
	CongestionWindow   int // sending congestion window
	SlowStartThreshold int // slow start size threshold, or -1 if the threshold is >= 0xFFFF
	// Generic like UDP, RAW
	Drops int
	// Related processes
	Procs    map[*ProcInfo]bool
	UserName string
}

func NewGenericRecord() *GenericRecord {
	t := new(GenericRecord)
	t.Procs = make(map[*ProcInfo]bool)
	return t
}

func GenericRecordRead(family string) (err error) {
	var (
		file        *os.File
		line        string
		fields      []string
		fieldsIndex int
		stringBuff  []string
		tempInt64   int64
	)
	switch family {
	case TCPv4Str:
		file, err = os.Open(TCPv4Path)
	case TCPv6Str:
		file, err = os.Open(TCPv6Path)
	case UDPv4Str:
		file, err = os.Open(UDPv4Path)
	case UDPv6Str:
		file, err = os.Open(UDPv6Path)
	default:
		err = fmt.Errorf("invalid family string.")
	}
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if err = scanner.Err(); err != nil {
			fmt.Println(err)
			return err
		}
		line = scanner.Text()
		fields = strings.Fields(line)
		if fields[0] == "sl" {
			continue
		}
		record := NewGenericRecord()
		// Local address
		fieldsIndex = 1
		stringBuff = strings.Split(fields[fieldsIndex], ":")
		switch family {
		case TCPv4Str, UDPv4Str:
			record.LocalAddr.Host, err = IPv4HexToString(stringBuff[0])
		case TCPv6Str, UDPv6Str:
			record.LocalAddr.Host, err = IPv6HexToString(stringBuff[0])
		}
		if err != nil {
			continue
		}
		if tempInt64, err = strconv.ParseInt(stringBuff[1], 16, 64); err != nil {
			fmt.Println(err)
			continue
		}
		record.LocalAddr.Port = fmt.Sprintf("%d", tempInt64)
		if MaxLocalAddrLength < len(record.LocalAddr.String()) {
			MaxLocalAddrLength = len(record.LocalAddr.String())
		}
		fieldsIndex++
		// Remote address
		stringBuff = strings.Split(fields[fieldsIndex], ":")
		switch family {
		case TCPv4Str, UDPv4Str:
			record.RemoteAddr.Host, err = IPv4HexToString(stringBuff[0])
		case TCPv6Str, UDPv6Str:
			record.RemoteAddr.Host, err = IPv6HexToString(stringBuff[0])
		}
		if err != nil {
			continue
		}
		if tempInt64, err = strconv.ParseInt(stringBuff[1], 16, 64); err != nil {
			fmt.Println(err)
			continue
		}
		record.RemoteAddr.Port = fmt.Sprintf("%d", tempInt64)
		if MaxRemoteAddrLength < len(record.RemoteAddr.String()) {
			MaxRemoteAddrLength = len(record.RemoteAddr.String())
		}
		fieldsIndex++
		// Status
		if tempInt64, err = strconv.ParseInt(fields[fieldsIndex], 16, 32); err != nil {
			fmt.Println(err)
			continue
		}
		record.Status = int(tempInt64)
		fieldsIndex++
		// TxQueue:RxQueue
		stringBuff = strings.Split(fields[fieldsIndex], ":")
		if tempInt64, err = strconv.ParseInt(stringBuff[0], 16, 32); err != nil {
			fmt.Println(err)
			continue
		}
		record.TxQueue = int(tempInt64)
		if tempInt64, err = strconv.ParseInt(stringBuff[1], 16, 32); err != nil {
			fmt.Println(err)
			continue
		}
		record.RxQueue = int(tempInt64)
		fieldsIndex++
		// Timer:TmWhen
		stringBuff = strings.Split(fields[fieldsIndex], ":")
		if tempInt64, err = strconv.ParseInt(stringBuff[0], 16, 32); err != nil {
			fmt.Println(err)
			continue
		}
		record.Timer = int(tempInt64)
		if record.Timer > 4 {
			record.Timer = 5
		}
		if tempInt64, err = strconv.ParseInt(stringBuff[1], 16, 32); err != nil {
			fmt.Println(err)
			continue
		}
		record.Timeout = int(tempInt64)
		fieldsIndex++
		// Retransmit
		if tempInt64, err = strconv.ParseInt(fields[fieldsIndex], 16, 32); err != nil {
			fmt.Println(err)
			continue
		}
		record.Retransmit = int(tempInt64)
		fieldsIndex++
		// UID
		if record.UID, err = strconv.ParseUint(fields[fieldsIndex], 10, 64); err != nil {
			fmt.Println(err)
			continue
		}
		fieldsIndex++
		// Timeout
		if record.Probes, err = strconv.Atoi(fields[fieldsIndex]); err != nil {
			fmt.Println(err)
			continue
		}
		fieldsIndex++
		// Inode
		if record.Inode, err = strconv.ParseUint(fields[fieldsIndex], 10, 64); err != nil {
			fmt.Println(err)
			continue
		}
		fieldsIndex++
		// Socket reference count
		if record.RefCount, err = strconv.Atoi(fields[fieldsIndex]); err != nil {
			fmt.Println(err)
			continue
		}
		fieldsIndex++
		if record.SK, err = strconv.ParseUint(fields[fieldsIndex], 16, 64); err != nil {
			fmt.Println(err)
			continue
		}
		switch family {
		case TCPv4Str, TCPv6Str:
			if record.Inode != 0 {
				fieldsIndex++
				if record.RTO, err = strconv.Atoi(fields[fieldsIndex]); err != nil {
					fmt.Println(err)
					continue
				}
				fieldsIndex++
				if record.ATO, err = strconv.Atoi(fields[fieldsIndex]); err != nil {
					fmt.Println(err)
					continue
				}
				fieldsIndex++
				if record.QACK, err = strconv.Atoi(fields[fieldsIndex]); err != nil {
					fmt.Println(err)
					continue
				}
				fieldsIndex++
				if record.CongestionWindow, err = strconv.Atoi(fields[fieldsIndex]); err != nil {
					fmt.Println(err)
					continue
				}
				fieldsIndex++
				if record.SlowStartThreshold, err = strconv.Atoi(fields[fieldsIndex]); err != nil {
					fmt.Println(err)
					continue
				}
			} else {
				record.RTO = 0
				record.ATO = 0
				record.QACK = 0
				record.CongestionWindow = 2
				record.SlowStartThreshold = -1
			}
		case UDPv4Str, UDPv6Str:
			fieldsIndex++
			if record.Drops, err = strconv.Atoi(fields[fieldsIndex]); err != nil {
				fmt.Println(err)
				continue
			}
		}
		switch family {
		case TCPv4Str:
			GlobalTCPv4Records[record.Inode] = record
		case TCPv6Str:
			GlobalTCPv6Records[record.Inode] = record
		case UDPv4Str:
			GlobalUDPv4Records[record.Inode] = record
		case UDPv6Str:
			GlobalUDPv6Records[record.Inode] = record
		}
	}
	return nil
}
