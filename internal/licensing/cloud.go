package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const cloudLeasePrefix = "XCL1."

type CloudLeaseClaims struct {
	Product          string `json:"product"`
	Machine          string `json:"machine"`
	Serial           string `json:"serial"`
	Challenge        string `json:"challenge,omitempty"`
	IssuedAt         int64  `json:"issued_at"`
	ExpiresAt        int64  `json:"expires_at"`
	LicenseExpiresAt int64  `json:"license_expires_at"`
}

func SignCloudLease(privateKeyBase64 string, claims CloudLeaseClaims) (string, error) {
	privateKey, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(privateKeyBase64))
	if err != nil || len(privateKey) != ed25519.PrivateKeySize {
		return "", errors.New("云授权签名私钥无效")
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signature := ed25519.Sign(ed25519.PrivateKey(privateKey), payload)
	return cloudLeasePrefix + base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func VerifyCloudLease(token, machine, serial string, localLicenseExpiresAt int64, now time.Time) (CloudLeaseClaims, error) {
	var claims CloudLeaseClaims
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, cloudLeasePrefix) {
		return claims, errors.New("云授权租约格式不正确")
	}
	parts := strings.Split(strings.TrimPrefix(token, cloudLeasePrefix), ".")
	if len(parts) != 2 {
		return claims, errors.New("云授权租约格式不正确")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return claims, errors.New("云授权租约载荷无法解析")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, errors.New("云授权租约签名无法解析")
	}
	publicKey, err := licensePublicKey()
	if err != nil || !ed25519.Verify(publicKey, payload, signature) {
		return claims, errors.New("云授权租约签名无效或已被篡改")
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return claims, errors.New("云授权租约内容无法解析")
	}
	if claims.Product != ProductID {
		return claims, errors.New("云授权租约不属于本插件")
	}
	if !strings.EqualFold(strings.TrimSpace(claims.Machine), strings.TrimSpace(machine)) {
		return claims, errors.New("云授权租约与本机不匹配")
	}
	if !strings.EqualFold(strings.TrimSpace(claims.Serial), strings.TrimSpace(serial)) {
		return claims, errors.New("云授权租约与当前卡密不匹配")
	}
	if claims.IssuedAt > now.Add(10*time.Minute).Unix() {
		return claims, errors.New("云授权租约签发时间异常")
	}
	if claims.ExpiresAt <= now.Unix() {
		return claims, errors.New("云授权租约已过期")
	}
	if claims.LicenseExpiresAt != localLicenseExpiresAt || claims.ExpiresAt > localLicenseExpiresAt {
		return claims, errors.New("云授权租约超出本地卡密有效期")
	}
	return claims, nil
}
