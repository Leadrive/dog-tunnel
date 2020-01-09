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
)

const BLOCK_SIZE = 1024
const START_FLAG = "start-->"
const END_FLAG = "<----end"

var fileName = toolbox.GetCurrentDirectory() + "/test.zip"

const taskId = 11

// 获取服务端发送的消息
func clientRead(conn net.Conn) int {
	buf := make([]byte, 12)
	n, err := conn.Read(buf)
	if err != nil {
		log.Fatalf("receive server info faild: %s\n", err)
	}
	// string conver int
	off, err := strconv.Atoi(strings.TrimSpace(string(buf[:n])))
	if err != nil {
		log.Fatalf("string conver int faild: %s\n", err)
	}
	return off
}

// 发送消息到服务端
func clientWrite(conn net.Conn, data []byte) {
	_, err := conn.Write(data)
	if err != nil {
		log.Fatalf("send content faild: %s\n", err)
	}
	// log.Printf("send 【%s】 content success\n", string(data))
}

var fileLen int64
var displayCount = 0

// client conn
func clientConn(conn net.Conn) {
	defer conn.Close()

	size := strconv.FormatInt(toolbox.GetFileSize(fileName), 10) // 文件可能被占用，读尺寸时出错
	md5 := toolbox.GetFileMd5(fileName)
	head := START_FLAG + fmt.Sprintf("%12v", strconv.Itoa(taskId)) + ":" + fmt.Sprintf("%12v", size) + ":" + md5
	log.Printf("len=%d, ", len(head), head)
	// 发送"start-->"消息通知服务端，我要开始发送文件内容了
	// 你赶紧告诉我你那边已经接收了多少内容,我从你已经接收的内容处开始继续发送
	clientWrite(conn, []byte(head))
	off := clientRead(conn)
	fileLen = int64(off)

	// send file content
	fp, err := os.OpenFile(fileName, os.O_RDONLY, 0755)
	defer fp.Close()
	if err != nil {
		log.Fatalf("open file faild: %s\n", err)
	}

	// set file seek
	// 设置从哪里开始读取文件内容
	_, err = fp.Seek(int64(off), 0)
	if err != nil {
		log.Fatalf("set file seek faild: %s\n", err)
	}
	log.Printf("read file at seek: %d\n", off)

	data := make([]byte, BLOCK_SIZE)
	for {
		// 每次发送定义的字节大小的内容
		n, err := fp.Read(data)
		log.Printf("Read n=%d", n)
		if err != nil {
			if err == io.EOF {
				// 如果已经读取完文件内容
				// 就发送'<----end'消息通知服务端，文件内容发送完了
				// time.Sleep(time.Second * 1)
				clientWrite(conn, []byte(END_FLAG))
				log.Printf("send all content, now quit, n=%d", n)
				break
			}
			log.Fatalf("read file err: %s\n", err)
		}
		// 发送文件内容到服务端

		fileLen += int64(n)
		if displayCount > 100 {
			log.Printf("Send fileLen=%d", fileLen)
			displayCount = 0
		}
		displayCount++
		clientWrite(conn, data[:n])
	}
	log.Printf("Send fileLen=%d", fileLen)
}

func main() {
	// connect timeout 10s
	conn, err := net.DialTimeout("tcp", ":8888", time.Second*10)
	if err != nil {
		log.Fatalf("client dial faild: %s\n", err)
	}
	clientConn(conn)
}
