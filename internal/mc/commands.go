package mc

import (
	"crypto/rand"
	"encoding/binary"
)

const CommandVersion = 10223616

func BuildSettingsCommand(command string, suppress bool) []byte {
	b := []byte{IDSettingsCommand, 1}
	b = append(b, WriteVarint(uint32(len(command)))...)
	b = append(b, []byte(command)...)
	if suppress {
		b = append(b, 1)
	} else {
		b = append(b, 0)
	}
	return b
}

func BuildCommandRequest(command, requestID string) []byte {
	b := []byte{IDCommandRequest}
	b = append(b, WriteString(command)...)
	b = append(b, WriteVarint(0)...)
	uid := make([]byte, 16)
	rand.Read(uid)
	b = append(b, uid...)
	b = append(b, WriteString(requestID)...)
	b = append(b, 0)
	b = append(b, WriteVarint(CommandVersion)...)
	b = append(b, 0)
	return b
}

func BuildTextPacket(textType int, sourceName, message string) []byte {
	b := []byte{IDText}
	b = append(b, WriteVarint(uint32(textType))...)
	b = append(b, WriteVarint(0)...)

	switch textType {
	case 1, 7, 8:
		b = append(b, WriteString(sourceName)...)
		b = append(b, WriteString(message)...)
	case 0, 5, 6:
		b = append(b, WriteString(message)...)
	default:
		b = append(b, WriteString(message)...)
		b = append(b, WriteVarint(0)...)
	}

	return b
}

func BuildRawCommand(cmd string) string {
	if len(cmd) > 0 && cmd[0] == '/' {
		return cmd
	}
	return "/" + cmd
}

var _ = binary.LittleEndian
