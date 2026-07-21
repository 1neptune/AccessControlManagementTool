package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
)

// encryptionKey AES加密密钥
// 使用16字节密钥（AES-128），用于加密服务器密码等敏感信息
// 注意：生产环境应从安全配置文件或环境变量中读取密钥
var encryptionKey = []byte("key-2026-01-01-a")

// Encrypt 使用AES-GCM算法加密文本
// 参数:
//   text - 待加密的明文文本
// 返回值:
//   string - Base64编码后的密文
//   error - 加密过程中的错误信息
// 加密流程:
//   1. 创建AES密码块（AES-128）
//   2. 创建GCM模式（Galois/Counter Mode），提供认证加密
//   3. 生成随机nonce（每次加密使用不同的nonce）
//   4. 使用GCM模式加密明文，将nonce附加在密文前面
//   5. 对加密结果进行Base64 URL安全编码
func Encrypt(text string) (string, error) {
	log.Printf("[Crypto] 开始加密操作，明文长度: %d 字节", len(text))

	log.Printf("[Crypto] 创建AES密码块")
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		log.Printf("[Crypto] AES密码块创建失败: %v", err)
		return "", err
	}

	log.Printf("[Crypto] 创建GCM模式")
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Printf("[Crypto] GCM模式创建失败: %v", err)
		return "", err
	}

	log.Printf("[Crypto] 生成随机nonce，长度: %d 字节", gcm.NonceSize())
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		log.Printf("[Crypto] nonce生成失败: %v", err)
		return "", err
	}

	log.Printf("[Crypto] 执行AES-GCM加密")
	ciphertext := gcm.Seal(nonce, nonce, []byte(text), nil)

	log.Printf("[Crypto] 执行Base64编码")
	result := base64.URLEncoding.EncodeToString(ciphertext)

	log.Printf("[Crypto] 加密完成，密文长度: %d 字节", len(result))
	return result, nil
}

// Decrypt 使用AES-GCM算法解密文本
// 参数:
//   ciphertext - Base64编码的密文
// 返回值:
//   string - 解密后的明文文本
//   error - 解密过程中的错误信息
// 解密流程:
//   1. 对密文进行Base64 URL安全解码
//   2. 创建AES密码块（AES-128）
//   3. 创建GCM模式
//   4. 从数据中分离nonce和实际密文
//   5. 使用GCM模式解密并验证数据完整性
//   6. 返回解密后的明文
// 安全特性:
//   - GCM模式提供认证加密，能检测数据篡改
//   - nonce长度检查防止短密文攻击
func Decrypt(ciphertext string) (string, error) {
	log.Printf("[Crypto] 开始解密操作，密文长度: %d 字节", len(ciphertext))

	log.Printf("[Crypto] 执行Base64解码")
	data, err := base64.URLEncoding.DecodeString(ciphertext)
	if err != nil {
		log.Printf("[Crypto] Base64解码失败: %v", err)
		return "", fmt.Errorf("base64解码失败: %v", err)
	}

	log.Printf("[Crypto] Base64解码成功，原始数据长度: %d 字节", len(data))

	log.Printf("[Crypto] 创建AES密码块")
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		log.Printf("[Crypto] AES密码块创建失败: %v", err)
		return "", err
	}

	log.Printf("[Crypto] 创建GCM模式")
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Printf("[Crypto] GCM模式创建失败: %v", err)
		return "", err
	}

	nonceSize := gcm.NonceSize()
	log.Printf("[Crypto] Nonce大小: %d 字节", nonceSize)

	if len(data) < nonceSize {
		log.Printf("[Crypto] 密文长度不足，需要至少 %d 字节，实际 %d 字节", nonceSize, len(data))
		return "", fmt.Errorf("密文长度不足")
	}

	nonce, ciphertextData := data[:nonceSize], data[nonceSize:]
	log.Printf("[Crypto] 分离nonce(%d字节)和密文数据(%d字节)", len(nonce), len(ciphertextData))

	log.Printf("[Crypto] 执行AES-GCM解密")
	plaintext, err := gcm.Open(nil, nonce, ciphertextData, nil)
	if err != nil {
		log.Printf("[Crypto] 解密失败: %v", err)
		return "", fmt.Errorf("解密失败: %v", err)
	}

	log.Printf("[Crypto] 解密完成，明文长度: %d 字节", len(plaintext))
	return string(plaintext), nil
}
