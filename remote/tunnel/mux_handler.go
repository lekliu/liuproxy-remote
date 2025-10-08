package tunnel

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/xtaci/smux"
	"liuproxy_remote/remote/core/securecrypt"
	"liuproxy_remote/remote/types"
)

// HandleMuxSession 负责处理一个基于 smux 的多路复用会话
func HandleMuxSession(conn net.Conn, reader *bufio.Reader, cfg *types.Config) {
	// 1. 根据 reader 是否为 nil，决定传给 smux.Server 的 io.ReadWriteCloser
	var smuxInput io.ReadWriteCloser = conn // 默认直接使用 conn
	if reader != nil {
		// 如果有预读的 reader，则使用 bufferedConn 包装器
		smuxInput = &bufferedConn{Conn: conn, reader: reader}
	}

	// 1. 将物理连接包装成 smux 服务端会话
	smuxConfig := smux.DefaultConfig()
	smuxConfig.Version = 2
	smuxConfig.KeepAliveInterval = 10 * time.Second
	smuxConfig.KeepAliveTimeout = 30 * time.Second

	session, err := smux.Server(smuxInput, smuxConfig)
	if err != nil {
		log.Printf("[REMOTE-MUX] Failed to create smux session: %v", err)
		conn.Close()
		return
	}
	defer session.Close()

	//log.Printf("[REMOTE-MUX] New smux session established from %s", conn.RemoteAddr())

	// 2. 在循环中接受逻辑流
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			//log.Printf("[REMOTE-MUX] Session from %s closed: %v", conn.RemoteAddr(), err)
			return // 会话关闭或出错
		}

		//log.Printf("[REMOTE-MUX] Accepted new stream %d from %s", stream.ID(), conn.RemoteAddr())

		// 为每个流启动一个 goroutine 进行处理
		go func(s *smux.Stream) {
			defer s.Close()
			handleMuxStream(s, cfg)
		}(stream)
	}
}

// handleMuxStream 处理单个 smux 逻辑流，其逻辑与 handleTCPStream 非常相似
func handleMuxStream(stream *smux.Stream, cfg *types.Config) {
	// 1. 创建加密器
	cipher, err := securecrypt.NewCipher(cfg.CommonConf.Crypt)
	if err != nil {
		log.Printf("[REMOTE-MUX-STREAM %d] Failed to create cipher: %v", stream.ID(), err)
		return
	}

	// 2. 读取并解密元数据包
	lenBuf := make([]byte, 2)
	if _, err := io.ReadFull(stream, lenBuf); err != nil {
		log.Printf("[REMOTE-MUX-STREAM %d] Failed to read metadata length: %v", stream.ID(), err)
		return
	}
	metaLen := binary.BigEndian.Uint16(lenBuf)
	encryptedMeta := make([]byte, metaLen)
	if _, err := io.ReadFull(stream, encryptedMeta); err != nil {
		log.Printf("[REMOTE-MUX-STREAM %d] Failed to read metadata payload: %v", stream.ID(), err)
		return
	}

	decryptedMetaBytes, err := cipher.Decrypt(encryptedMeta)
	if err != nil {
		log.Printf("[REMOTE-MUX-STREAM %d] Failed to decrypt metadata: %v", stream.ID(), err)
		return
	}

	meta, err := ReadMetadata(bytes.NewReader(decryptedMetaBytes))
	if err != nil {
		log.Printf("[REMOTE-MUX-STREAM %d] Failed to parse metadata: %v", stream.ID(), err)
		return
	}

	//log.Printf("[REMOTE-MUX-STREAM %d] Metadata parsed. Target: %s:%d", stream.ID(), meta.Addr, meta.Port)

	// 3. 连接最终目标
	targetAddr := net.JoinHostPort(meta.Addr, strconv.Itoa(meta.Port))
	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("[REMOTE-MUX-STREAM %d] Failed to dial target %s: %v", stream.ID(), targetAddr, err)
		return
	}
	defer targetConn.Close()

	// 4. 启动双向转发 (逻辑与 tcp_handler.go 完全相同)
	var wg sync.WaitGroup
	wg.Add(2)

	// Uplink (stream -> target)
	go func() {
		defer wg.Done()
		for {
			lenBuf := make([]byte, 2)
			_, err := io.ReadFull(stream, lenBuf)
			if err != nil {
				break
			}
			payloadLen := binary.BigEndian.Uint16(lenBuf)
			buf := make([]byte, payloadLen)
			_, err = io.ReadFull(stream, buf)
			if err != nil {
				break
			}
			decrypted, dErr := cipher.Decrypt(buf)
			if dErr != nil {
				break
			}
			if _, wErr := targetConn.Write(decrypted); wErr != nil {
				break
			}
		}
		if tcpConn, ok := targetConn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	// Downlink (target -> stream)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		lenBuf := make([]byte, 2)
		for {
			n, err := targetConn.Read(buf)
			if n > 0 {
				encrypted, eErr := cipher.Encrypt(buf[:n])
				if eErr != nil {
					break
				}
				binary.BigEndian.PutUint16(lenBuf, uint16(len(encrypted)))
				if _, wErr := stream.Write(lenBuf); wErr != nil {
					break
				}
				if _, wErr := stream.Write(encrypted); wErr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		stream.Close() // CloseWrite() is not available on smux.Stream
	}()

	wg.Wait()
	//log.Printf("[REMOTE-MUX-STREAM %d] Relay finished for target %s.", stream.ID(), targetAddr)
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

// Read 方法优先从 bufio.Reader 中读取数据，然后再从原始连接中读取
func (b *bufferedConn) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}
