// +build ignore
package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	toolbox "code.aliyun.com/luocanqi/go_toolbox"
	// "time"
)

const HEAD_SIZE = 66

var BLOCK_SIZE = HEAD_SIZE

const START_FLAG = "start-->"
const END_FLAG = "<----end"
const fileName = "test_1.zip"

// 把接收到的内容append到文件
func writeFile(fp *os.File, content []byte) {
	if len(content) != 0 {
		_, err := fp.Write(content)
		if err != nil {
			log.Fatalf("append content to file faild: %s\n", err)
		}
		// log.Printf("append content: 【%s】 success\n", string(content))
	}
}

// 获取已接收内容的大小
// (断点续传需要把已接收内容大下通知客户端从哪里开始发送文件内容)
func getFileStat() int64 {
	fileinfo, err := os.Stat(fileName)
	if err != nil {
		// 如果首次没有创建test_1.txt文件，则直接返回0
		// 告诉客户端从头开始发送文件内容
		if os.IsNotExist(err) {
			log.Printf("file size: %d\n", 0)
			return int64(0)
		}
		log.Fatalf("get file stat faild: %s\n", err)
	}
	log.Printf("file size: %d\n", fileinfo.Size())
	return fileinfo.Size()
}

var fileLen int64
var fileSize int64
var md5 string
var displayCount = 0

//https://github.com/howeyc/crc16/blob/master/crc16.go

// Read reads all of data or returns an error
func Read(conn net.Conn, data []byte) (int, error) {
	index := 0
	try := 0
	for index < len(data) {
		n, err := conn.Read(data[index:])
		if err != nil {
			log.Printf("Read n=%d, error=%s", n, err)
			e, ok := err.(net.Error)
			if !ok || !e.Temporary() || try >= 3 {
				return 0, err
			}
			try++
		}
		index += n
	}
	return index, nil
}

func serverConn(conn net.Conn) error {
	defer conn.Close()
	fp, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0755)
	defer fp.Close()
	if err != nil {
		log.Printf("open file faild: %s\n", err)
		return err
	}
	off, err := fp.Seek(0, os.SEEK_END)
	if err != nil {
		log.Printf("seek file faild: %s\n", err)
		return err
	}
	fmt.Println("offset:", off)
	var buf = make([]byte, BLOCK_SIZE)
	for {
		// 需要设置超时，否则客户端可能断开了
		if err = conn.SetReadDeadline(time.Now().Add(time.Second * 10)); err != nil {
			log.Printf("SetReadDeadline, err=%s\n", err)
			return err
		}
		n, err := Read(conn, buf)
		// n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				log.Println("server io EOF, n=%d\n", n)
				break
			}
			log.Fatalf("server read faild: %s\n", err)
		}
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			log.Printf("SetReadDeadline, err=%s\n", err)
			return err
		}
		// log.Printf("recevice %d bytes, content is 【%s】\n", n, string(buf[:n]))
		// 判断客户端发送过来的消息
		// 如果是’start-->‘则表示需要告诉客户端从哪里开始读取文件数据发送
		if n == HEAD_SIZE {
			log.Printf(string(buf[:n]))
			switch string(buf[:8]) {
			case START_FLAG: // 后面跟的是文件size
				arr := strings.Split(string(buf[8:n]), ":") // 分解出文件长度和MD5
				taskId, _ := strconv.Atoi(strings.TrimSpace(arr[0]))
				log.Printf("taskId:%d", taskId)
				size, err := strconv.ParseInt(strings.TrimSpace(arr[1]), 10, 64)
				if err != nil {
					log.Printf("get file size fail: %s", err)
				}
				fileSize = size
				md5 = arr[2]
				log.Printf("size:%d, md5:%s", fileSize, md5)
				// off := getFileStat()
				// int conver string
				// stringoff := strconv.FormatInt(off, 10)
				stringoff := fmt.Sprintf("%12v", off)  // 跟client端保持接收长度为12，否则Read函数一直在等
				_, err = conn.Write([]byte(stringoff)) // 发送传输开始位置给客户端
				if err != nil {
					log.Printf("server write fail: %s", err)
				}
				BLOCK_SIZE = 1024
				buf = make([]byte, BLOCK_SIZE)
				continue
			}
		}
		if n != BLOCK_SIZE && n >= 8 { // 结束标识，会跟内容粘在一起
			if n == 8 && string(buf[:8]) == END_FLAG {
				// 如果接收到客户端通知所有文件内容发送完毕消息则退出
				log.Printf("receive over")
				break
			} else if string(buf[n-8:n]) == END_FLAG {
				n = n - 8
			}
		}
		// 把客户端发送的内容保存到文件
		fileLen += int64(n)
		if displayCount > 100 {
			log.Printf("fileLen=%d", fileLen)
			displayCount = 0
		}
		displayCount++
		writeFile(fp, buf[:n])
	}
	log.Printf("fileLen=%d, receive md5=%s, file md5=%s", fileLen, md5, toolbox.GetFileMd5(fileName))
	if fileLen == fileSize {
		if toolbox.GetFileMd5(fileName) == md5 {
			log.Printf("md5 verify ok")
		} else {
			log.Printf("md5 verify fail")
		}
	} else {
		log.Printf("Receive break")
	}
	return nil
}

func main() {
	// 建立监听
	l, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatalf("error listen: %s\n", err)
	}
	defer l.Close()

	// 允许客户端连接，在没有客户端连接时，会一直阻塞
	for {
		log.Println("waiting accept.")
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("accept faild: %s\n", err)
		}
		go serverConn(conn)
	}
}
