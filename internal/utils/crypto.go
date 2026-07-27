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

// encryptionKey AES加密密钥，长度为16字节（AES-128）
// 该密钥用于加密服务器密码等敏感信息，确保数据库中存储的密码不为明文
// 注意点:
//   - 当前密钥硬编码在代码中，仅用于演示和测试目的
//   - 生产环境应从安全配置文件、环境变量或密钥管理服务中读取密钥
//   - 密钥长度必须为16、24或32字节，分别对应AES-128、AES-192、AES-256
//   - 密钥变更会导致所有历史加密数据无法解密，需谨慎管理
var encryptionKey = []byte("key-2026-01-01-a")

// Encrypt 使用AES-GCM算法加密文本
// 将明文文本加密为Base64编码的密文，用于保护敏感信息
// 参数:
//   text - 待加密的明文文本，如服务器密码
// 返回值:
//   string - Base64 URL安全编码后的密文，可直接存储到数据库
//   error - 加密过程中的错误信息，成功返回nil
// 加密流程:
//   1. 创建AES密码块（使用AES-128算法，因为密钥长度为16字节）
//   2. 创建GCM模式（Galois/Counter Mode），提供认证加密功能
//   3. 生成随机nonce（每次加密使用不同的nonce，防止相同明文产生相同密文）
//   4. 使用GCM模式加密明文，加密结果包含nonce和认证标签
//   5. 对加密结果进行Base64 URL安全编码，便于存储和传输
// 安全特性:
//   - GCM模式提供认证加密，能检测数据是否被篡改
//   - 每次加密使用随机nonce，防止重放攻击和频率分析
//   - Base64 URL编码避免特殊字符问题，适合存储到数据库
// 注意点:
//   - 加密密钥必须保密，泄露密钥将导致所有加密数据可被解密
//   - nonce不应重复使用，本实现使用crypto/rand生成安全随机数
//   - 加密后的密文长度会比明文长，因为包含nonce和认证标签
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
// 将Base64编码的密文解密为明文文本，用于恢复加密的敏感信息
// 参数:
//   ciphertext - Base64 URL安全编码的密文
// 返回值:
//   string - 解密后的明文文本，如服务器密码
//   error - 解密过程中的错误信息，成功返回nil
// 解密流程:
//   1. 对密文进行Base64 URL安全解码，获取原始字节数据
//   2. 创建AES密码块（必须与加密时使用相同的密钥）
//   3. 创建GCM模式
//   4. 从数据中分离nonce（前12字节）和实际密文数据
//   5. 使用GCM模式解密并验证数据完整性和真实性
//   6. 返回解密后的明文
// 安全特性:
//   - GCM模式提供认证解密，能检测数据篡改和完整性破坏
//   - nonce长度检查防止短密文攻击和格式错误
//   - 解密失败返回明确的错误信息，便于问题定位
// 注意点:
//   - 解密必须使用与加密相同的密钥，否则会失败
//   - 密文格式必须正确（Base64 URL编码，包含有效的nonce和认证标签）
//   - 如果密文被篡改，解密会失败并返回错误
//   - 解密后的明文应妥善处理，避免在内存中长期保留
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
