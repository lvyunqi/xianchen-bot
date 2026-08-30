package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	ProductID       = "xianlv-bee-plugin"
	publicKeyBase64 = "wfOIyzSkBKq5cLT9w5OsGv4-NhjI__IMvFJSJhAhuIU"
	compactPrefix   = "XC2."
	compactVersion  = byte(2)
	compactBodySize = 27
)

type Claims struct {
	Product   string `json:"product"`
	Machine   string `json:"machine"`
	IssuedAt  int64  `json:"issued_at"`
	ExpiresAt int64  `json:"expires_at"`
	Nonce     string `json:"nonce"`
}

func MachineCode() string {
	hostname, _ := os.Hostname()
	parts := []string{
		strings.ToUpper(strings.TrimSpace(hostname)),
		strings.ToUpper(strings.TrimSpace(os.Getenv("COMPUTERNAME"))),
		strings.ToUpper(strings.TrimSpace(os.Getenv("PROCESSOR_IDENTIFIER"))),
		strings.ToUpper(strings.TrimSpace(os.Getenv("SystemDrive"))),
		runtime.GOOS,
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "|")))
	encoded := strings.ToUpper(hex.EncodeToString(digest[:10]))
	return strings.Join([]string{encoded[0:5], encoded[5:10], encoded[10:15], encoded[15:20]}, "-")
}

func Verify(code, machine string, now time.Time) (Claims, error) {
	code = strings.TrimSpace(code)
	if len(code) >= len(compactPrefix) && strings.EqualFold(code[:len(compactPrefix)], compactPrefix) {
		return verifyCompact(code[len(compactPrefix):], machine, now)
	}
	return verifyLegacy(code, machine, now)
}

func verifyLegacy(code, machine string, now time.Time) (Claims, error) {
	var claims Claims
	parts := strings.Split(code, ".")
	if len(parts) != 2 {
		return claims, errors.New("卡密格式不正确")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return claims, errors.New("卡密载荷无法解析")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, errors.New("卡密签名无法解析")
	}
	publicKey, err := licensePublicKey()
	if err != nil {
		return claims, errors.New("插件授权公钥损坏")
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return claims, errors.New("卡密签名无效或已被篡改")
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return claims, errors.New("卡密内容无法解析")
	}
	return validateClaims(claims, machine, now)
}

func Generate(privateKeyBase64, machine string, issuedAt, expiresAt time.Time) (string, Claims, error) {
	var claims Claims
	privateKey, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(privateKeyBase64))
	if err != nil || len(privateKey) != ed25519.PrivateKeySize {
		return "", claims, errors.New("生成器私钥无效")
	}
	machine = strings.ToUpper(strings.TrimSpace(machine))
	if machine == "" {
		return "", claims, errors.New("机器码不能为空")
	}
	if !expiresAt.After(issuedAt) {
		return "", claims, errors.New("到期时间必须晚于签发时间")
	}
	if issuedAt.Unix() < 0 || expiresAt.Unix() < 0 || issuedAt.Unix() > int64(^uint32(0)) || expiresAt.Unix() > int64(^uint32(0)) {
		return "", claims, errors.New("授权时间超出紧凑卡密支持范围")
	}
	nonceBytes := make([]byte, 8)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", claims, err
	}
	claims = Claims{Product: ProductID, Machine: machine, IssuedAt: issuedAt.Unix(), ExpiresAt: expiresAt.Unix(), Nonce: hex.EncodeToString(nonceBytes)}
	payload := make([]byte, compactBodySize)
	payload[0] = compactVersion
	machineDigest := sha256.Sum256([]byte(machine))
	copy(payload[1:11], machineDigest[:10])
	binary.BigEndian.PutUint32(payload[11:15], uint32(claims.IssuedAt))
	binary.BigEndian.PutUint32(payload[15:19], uint32(claims.ExpiresAt))
	copy(payload[19:27], nonceBytes)
	signature := ed25519.Sign(ed25519.PrivateKey(privateKey), payload)
	code := compactPrefix + base64.RawURLEncoding.EncodeToString(append(payload, signature...))
	return code, claims, nil
}

func verifyCompact(encoded, machine string, now time.Time) (Claims, error) {
	var claims Claims
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(data) != compactBodySize+ed25519.SignatureSize {
		return claims, errors.New("紧凑卡密格式不正确")
	}
	payload, signature := data[:compactBodySize], data[compactBodySize:]
	publicKey, err := licensePublicKey()
	if err != nil {
		return claims, errors.New("插件授权公钥损坏")
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return claims, errors.New("卡密签名无效或已被篡改")
	}
	if payload[0] != compactVersion {
		return claims, errors.New("卡密版本不受支持")
	}
	machine = strings.ToUpper(strings.TrimSpace(machine))
	machineDigest := sha256.Sum256([]byte(machine))
	if subtle.ConstantTimeCompare(payload[1:11], machineDigest[:10]) != 1 {
		return claims, errors.New("卡密与本机机器码不匹配")
	}
	claims = Claims{
		Product:   ProductID,
		Machine:   machine,
		IssuedAt:  int64(binary.BigEndian.Uint32(payload[11:15])),
		ExpiresAt: int64(binary.BigEndian.Uint32(payload[15:19])),
		Nonce:     hex.EncodeToString(payload[19:27]),
	}
	return validateClaims(claims, machine, now)
}

func validateClaims(claims Claims, machine string, now time.Time) (Claims, error) {
	if claims.Product != ProductID {
		return claims, errors.New("卡密不属于本插件")
	}
	if !strings.EqualFold(strings.TrimSpace(claims.Machine), strings.TrimSpace(machine)) {
		return claims, errors.New("卡密与本机机器码不匹配")
	}
	if claims.IssuedAt > now.Add(10*time.Minute).Unix() {
		return claims, errors.New("卡密签发时间异常")
	}
	if claims.ExpiresAt <= now.Unix() {
		return claims, errors.New("卡密已到期")
	}
	if strings.TrimSpace(claims.Nonce) == "" {
		return claims, errors.New("卡密缺少唯一序号")
	}
	return claims, nil
}

func licensePublicKey() (ed25519.PublicKey, error) {
	publicKey, err := base64.RawURLEncoding.DecodeString(publicKeyBase64)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return nil, errors.New("invalid public key")
	}
	return ed25519.PublicKey(publicKey), nil
}

func generateLegacy(privateKeyBase64, machine string, issuedAt, expiresAt time.Time) (string, Claims, error) {
	var claims Claims
	privateKey, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(privateKeyBase64))
	if err != nil || len(privateKey) != ed25519.PrivateKeySize {
		return "", claims, errors.New("生成器私钥无效")
	}
	machine = strings.ToUpper(strings.TrimSpace(machine))
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", claims, err
	}
	claims = Claims{Product: ProductID, Machine: machine, IssuedAt: issuedAt.Unix(), ExpiresAt: expiresAt.Unix(), Nonce: hex.EncodeToString(nonceBytes)}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", claims, err
	}
	signature := ed25519.Sign(ed25519.PrivateKey(privateKey), payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature), claims, nil
}

func Summary(claims Claims) string {
	return fmt.Sprintf("机器码: %s; 到期: %s; 授权序号: %s", claims.Machine, time.Unix(claims.ExpiresAt, 0).Format("2006-01-02 15:04:05"), Serial(claims))
}

func Serial(claims Claims) string {
	nonce, err := hex.DecodeString(strings.TrimSpace(claims.Nonce))
	if err != nil || len(nonce) == 0 {
		digest := sha256.Sum256([]byte(claims.Nonce))
		nonce = digest[:]
	}
	serial := strings.TrimRight(base64.RawURLEncoding.EncodeToString(nonce), "=")
	serial = strings.ToUpper(strings.NewReplacer("-", "X", "_", "Y").Replace(serial))
	if len(serial) < 12 {
		digest := sha256.Sum256(nonce)
		serial += strings.ToUpper(base64.RawURLEncoding.EncodeToString(digest[:]))
	}
	return serial[:12]
}
