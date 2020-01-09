// +build ignore
package main

import (
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	toolbox "code.aliyun.com/luocanqi/go_toolbox"
	// "time"
)

const HEAD_SIZE = 66

var BLOCK_SIZE = HEAD_SIZE

const START_FLAG = "start-->"
const END_FLAG = "<----end"
const fileName = "test_1.zip"

// 把接收到的内容append到文件
func writeFile(content []byte) {
	if len(content) != 0 {
		fp, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0755)
		defer fp.Close()
		if err != nil {
			log.Fatalf("open file faild: %s\n", err)
		}
		_, err = fp.Write(content)
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

func serverConn(conn net.Conn) {
	defer conn.Close()
	for {
		var buf = make([]byte, BLOCK_SIZE)
		log.Println("begin read")
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				log.Println("server io EOF\n")
				return
			}
			log.Fatalf("server read faild: %s\n", err)
		}
		// log.Printf("recevice %d bytes, content is 【%s】\n", n, string(buf[:n]))
		// 判断客户端发送过来的消息
		// 如果是’start-->‘则表示需要告诉客户端从哪里开始读取文件数据发送
		if n == HEAD_SIZE {
			log.Printf(string(buf[:n]))
			switch string(buf[:8]) {
			case START_FLAG: // 后面跟的是文件size
				arr := strings.Split(string(buf[8:n]), ":") // 分解出文件长度和MD5
				taskId, _ := strconv.Atoi(strings.Trim(arr[0], ""))
				log.Printf("taskId:%d", taskId)
				size, err := strconv.ParseInt(strings.Trim(arr[1], ""), 10, 64)
				if err != nil {
					log.Printf("get file size fail: %s", err)
				}
				fileSize = size
				md5 = arr[2]
				log.Printf("size:%d, md5:%s", fileSize, md5)
				off := getFileStat()
				// int conver string
				stringoff := strconv.FormatInt(off, 10)
				_, err = conn.Write([]byte(stringoff)) // 发送传输开始位置给客户端
				if err != nil {
					log.Printf("server write fail: %s", err)
				}
				BLOCK_SIZE = 1024
				// continue
				return
			}
		}
		if n != BLOCK_SIZE && n >= 8 { // 结束标识，会跟内容粘在一起
			if n == 8 && string(buf[:8]) == END_FLAG {
				// 如果接收到客户端通知所有文件内容发送完毕消息则退出
				log.Printf("receive over")
				return
				// default:
				//     time.Sleep(time.Second * 1)
			} else if string(buf[n-8:n]) == END_FLAG {
				n = n - 8
			}
		}
		switch string(buf[:n]) {
		case START_FLAG:
			off := getFileStat()
			// int conver string
			stringoff := strconv.FormatInt(off, 10)
			_, err = conn.Write([]byte(stringoff))
			if err != nil {
				log.Fatalf("server write faild: %s\n", err)
			}
			fileLen = off
			continue
		case END_FLAG:
			// 如果接收到客户端通知所有文件内容发送完毕消息则退出
			log.Fatalf("receive over\n")
			if toolbox.GetFileMd5(fileName) != md5 {
				log.Printf("md5 verify ok")
			} else {
				log.Printf("md5 verify fail")
			}
			return
			// default:
			//     time.Sleep(time.Second * 1)
		}
		// 把客户端发送的内容保存到文件
		fileLen += int64(n)
		if displayCount > 10 {
			log.Printf("fileLen=%d", fileLen)
			displayCount = 0
		}
		displayCount++
		writeFile(buf[:n])
	}
	log.Printf("fileLen=%d", fileLen)
}

func main() {
	// 建立监听
	l, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatalf("error listen: %s\n", err)
	}
	defer l.Close()

	log.Println("waiting accept.")
	// 允许客户端连接，在没有客户端连接时，会一直阻塞
	conn, err := l.Accept()
	if err != nil {
		log.Fatalf("accept faild: %s\n", err)
	}
	log.Printf("begin receive data.")
	serverConn(conn)
}
