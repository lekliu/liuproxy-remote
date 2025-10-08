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

	"liuproxy_remote/remote/core/securecrypt"
	"liuproxy_remote/remote/types"
)

// handleTCPStream 修改签名以接收 bufio.Reader
func handleTCPStream(inboundConn net.Conn, reader *bufio.Reader, cfg *types.Config) {
	// 1. 创建加密器
	cipher, err := securecrypt.NewCipher(cfg.CommonConf.Crypt)
	if err != nil {
		log.Printf("[REMOTE-TCP-DIAG] Failed to create cipher: %v", err)
		return
	}

	// 2. 读取并解密第一个元数据包
	//log.Printf("[REMOTE-TCP-DIAG] Reading encrypted metadata from inbound connection...")
	lenBuf := make([]byte, 2)
	if _, err := io.ReadFull(reader, lenBuf); err != nil {
		log.Printf("[REMOTE-TCP-DIAG] Failed to read metadata length header: %v", err)
		return
	}
	metaLen := binary.BigEndian.Uint16(lenBuf)

	encryptedMeta := make([]byte, metaLen)
	if _, err := io.ReadFull(reader, encryptedMeta); err != nil {
		log.Printf("[REMOTE-TCP-DIAG] Failed to read encrypted metadata payload: %v", err)
		return
	}

	decryptedMetaBytes, err := cipher.Decrypt(encryptedMeta)
	if err != nil {
		log.Printf("[REMOTE-TCP-DIAG] Failed to decrypt metadata: %v", err)
		return
	}

	meta, err := ReadMetadata(bytes.NewReader(decryptedMetaBytes))
	if err != nil {
		log.Printf("[REMOTE-TCP-DIAG] Failed to parse decrypted metadata: %v", err)
		return
	}
	//log.Printf("[REMOTE-TCP-DIAG] Metadata decrypted successfully. StreamType: 0x%02x, Target: %s:%d", meta.Type, meta.Addr, meta.Port)

	// 根据元数据中的流类型，理论上可以处理TCP或UDP，但此函数只处理TCP
	if meta.Type != StreamTCP {
		log.Printf("[REMOTE-TCP-DIAG] Received non-TCP stream type 0x%02x in a TCP connection. Closing.", meta.Type)
		return
	}

	targetAddr := net.JoinHostPort(meta.Addr, strconv.Itoa(meta.Port))

	// 3. 连接最终目标
	//log.Printf("[REMOTE-TCP-DIAG] Dialing target: %s", targetAddr)
	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("[REMOTE-TCP-DIAG] Failed to dial target %s: %v", targetAddr, err)
		return
	}
	defer targetConn.Close()
	//log.Printf("[REMOTE-TCP-DIAG] Successfully dialed target: %s", targetAddr)

	// 4. 启动双向加密转发
	//log.Printf("[REMOTE-TCP-DIAG] Starting bidirectional relay for TCP stream.")
	var wg sync.WaitGroup
	wg.Add(2)

	// Uplink (inbound -> target)
	go func() {
		defer wg.Done()
		// Uplink 的 reader 现在是 MultiReader，它会先读取 bufio.Reader 中剩余的数据，然后再读取原始的 conn
		combinedReader := io.MultiReader(reader, inboundConn)
		lenBuf := make([]byte, 2)
		for {
			_, err := io.ReadFull(combinedReader, lenBuf) // 从组合 reader 读取
			if err != nil {
				if err != io.EOF && err != io.ErrUnexpectedEOF {
					//log.Printf("[REMOTE-TCP-UPLINK] Read length header failed: %v", err)
				}
				break
			}
			payloadLen := binary.BigEndian.Uint16(lenBuf)
			if payloadLen == 0 {
				continue
			}

			buf := make([]byte, payloadLen)
			_, err = io.ReadFull(combinedReader, buf) // 从组合 reader 读取
			if err != nil {
				log.Printf("[REMOTE-TCP-UPLINK] Read payload failed: %v", err)
				break
			}
			decrypted, dErr := cipher.Decrypt(buf)
			if dErr != nil {
				log.Printf("[REMOTE-TCP-UPLINK] Decryption failed: %v", dErr)
				break
			}
			if _, wErr := targetConn.Write(decrypted); wErr != nil {
				log.Printf("[REMOTE-TCP-UPLINK] Write to target failed: %v", wErr)
				break
			}
		}
		if tcpConn, ok := targetConn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	// Downlink (target -> inbound)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		lenBuf := make([]byte, 2)
		for {
			n, err := targetConn.Read(buf)
			if n > 0 {
				encrypted, eErr := cipher.Encrypt(buf[:n])
				if eErr != nil {
					log.Printf("[REMOTE-TCP-DOWNLINK] Encryption failed: %v", eErr)
					break
				}

				binary.BigEndian.PutUint16(lenBuf, uint16(len(encrypted)))
				if _, wErr := inboundConn.Write(lenBuf); wErr != nil {
					log.Printf("[REMOTE-TCP-DOWNLINK] Write length header failed: %v", wErr)
					break
				}

				if _, wErr := inboundConn.Write(encrypted); wErr != nil {
					log.Printf("[REMOTE-TCP-DOWNLINK] Write payload failed: %v", wErr)
					break
				}
			}
			if err != nil {
				//log.Printf("[REMOTE-TCP-DOWNLINK] Read from target finished: %v", err)
				break
			}
		}
		if tcpConn, ok := inboundConn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	wg.Wait()
	//log.Printf("[REMOTE-TCP-DIAG] Relay finished for target %s.", targetAddr)
}
