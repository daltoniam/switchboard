package mysqltest

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
)

const Password = "mysql-ui-fixture-password"

type Fixture struct{ Port string }

func Start(t *testing.T) *Fixture {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	var mutex sync.Mutex
	connections := map[net.Conn]bool{}
	var workers sync.WaitGroup
	workers.Add(1)
	t.Cleanup(func() {
		_ = listener.Close()
		mutex.Lock()
		for conn := range connections {
			_ = conn.Close()
		}
		mutex.Unlock()
		workers.Wait()
	})
	go func() {
		defer workers.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mutex.Lock()
			connections[conn] = true
			mutex.Unlock()
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer func() { _ = conn.Close() }()
				serve(conn)
				mutex.Lock()
				delete(connections, conn)
				mutex.Unlock()
			}()
		}
	}()
	return &Fixture{Port: port}
}

func packet(conn net.Conn, sequence byte, data []byte) error {
	header := []byte{byte(len(data)), byte(len(data) >> 8), byte(len(data) >> 16), sequence}
	_, err := conn.Write(append(header, data...))
	return err
}

func readPacket(conn net.Conn) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	length := int(header[0]) | int(header[1])<<8 | int(header[2])<<16
	if length > 1<<20 {
		return nil, fmt.Errorf("fixture packet too large")
	}
	data := make([]byte, length)
	_, err := io.ReadFull(conn, data)
	return data, err
}

func serve(conn net.Conn) {
	salt := []byte("12345678abcdefghijkl")
	capabilities := uint32(1 | 4 | 8 | 512 | 8192 | 32768 | 1<<19)
	hello := []byte{10}
	hello = append(hello, []byte("8.4.11-fixture\x00")...)
	hello = binary.LittleEndian.AppendUint32(hello, 1)
	hello = append(hello, salt[:8]...)
	hello = append(hello, 0)
	hello = binary.LittleEndian.AppendUint16(hello, uint16(capabilities))
	hello = append(hello, 45, 2, 0)
	hello = binary.LittleEndian.AppendUint16(hello, uint16(capabilities>>16))
	hello = append(hello, 21)
	hello = append(hello, make([]byte, 10)...)
	hello = append(hello, salt[8:]...)
	hello = append(hello, 0)
	hello = append(hello, []byte("mysql_native_password\x00")...)
	if packet(conn, 0, hello) != nil {
		return
	}
	auth, err := readPacket(conn)
	if err != nil || len(auth) < 34 {
		return
	}
	userEnd := bytes.IndexByte(auth[32:], 0)
	if userEnd < 0 {
		return
	}
	user := string(auth[32 : 32+userEnd])
	offset := 33 + userEnd
	if offset >= len(auth) {
		return
	}
	length := int(auth[offset])
	offset++
	if offset+length > len(auth) {
		return
	}
	token, err := hex.DecodeString("b3fd5a2c09797c6cacc549d0a91864659101f695")
	if err != nil {
		return
	}
	if user != "reader" || !bytes.Equal(auth[offset:offset+length], token) {
		_ = packet(conn, 2, append([]byte{255, 21, 4, '#'}, []byte("28000Fixture access denied")...))
		return
	}
	if packet(conn, 2, []byte{0, 0, 0, 2, 0, 0, 0}) != nil {
		return
	}
	for {
		request, err := readPacket(conn)
		if err != nil || len(request) == 0 {
			return
		}
		if request[0] == 1 {
			return
		}
		if request[0] == 14 {
			if packet(conn, 1, []byte{0, 0, 0, 2, 0, 0, 0}) != nil {
				return
			}
			continue
		}
		if request[0] != 3 {
			return
		}
		query := strings.ToUpper(strings.TrimSpace(string(request[1:])))
		switch {
		case strings.HasPrefix(query, "SET "), strings.HasPrefix(query, "START TRANSACTION"), query == "ROLLBACK", query == "COMMIT":
			if packet(conn, 1, []byte{0, 0, 0, 2, 0, 0, 0}) != nil {
				return
			}
		case query == "SELECT VERSION()":
			if result(conn, []string{"version"}, [][]string{{"8.4.11-fixture"}}) != nil {
				return
			}
		case strings.Contains(query, "INFORMATION_SCHEMA.SCHEMATA"):
			if result(conn, []string{"database_name", "character_set", "collation"}, [][]string{{"app", "utf8mb4", "utf8mb4_0900_ai_ci"}}) != nil {
				return
			}
		case query == "SELECT NAME FROM USERS":
			if result(conn, []string{"name"}, [][]string{{"MySQL UI fixture"}}) != nil {
				return
			}
		default:
			if packet(conn, 1, append([]byte{255, 40, 4, '#'}, []byte("42000Unsupported fixture query")...)) != nil {
				return
			}
		}
	}
}

func text(data []byte, value string) []byte {
	data = append(data, byte(len(value)))
	return append(data, value...)
}

func result(conn net.Conn, columns []string, rows [][]string) error {
	sequence := byte(1)
	if err := packet(conn, sequence, []byte{byte(len(columns))}); err != nil {
		return err
	}
	for _, name := range columns {
		sequence++
		var column []byte
		for _, value := range []string{"def", "app", "", "", name, ""} {
			column = text(column, value)
		}
		column = append(column, 12, 45, 0, 255, 0, 0, 0, 253, 0, 0, 0, 0, 0)
		if err := packet(conn, sequence, column); err != nil {
			return err
		}
	}
	sequence++
	if err := packet(conn, sequence, []byte{254, 0, 0, 2, 0}); err != nil {
		return err
	}
	for _, row := range rows {
		sequence++
		var data []byte
		for _, value := range row {
			data = text(data, value)
		}
		if err := packet(conn, sequence, data); err != nil {
			return err
		}
	}
	return packet(conn, sequence+1, []byte{254, 0, 0, 2, 0})
}
