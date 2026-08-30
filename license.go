package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xianlv/internal/licensing"
)

const licenseFilename = "license.key"

func validateRuntimeLicense(dataDir string) error {
	if strings.TrimSpace(os.Getenv("XIANLV_DEV_LICENSE")) == "1" {
		return nil
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	machine := licensing.MachineCode()
	_ = writeLicenseHelp(dataDir, machine)
	path := filepath.Join(dataDir, licenseFilename)
	code, err := os.ReadFile(path)
	if err != nil {
		securityLog(dataDir, "授权失败", "未找到license.key; machine="+machine)
		return fmt.Errorf("插件尚未授权，机器码 %s；请打开插件设置并粘贴授权卡密", machine)
	}
	now := time.Now()
	card := strings.TrimSpace(string(code))
	claims, err := licensing.Verify(card, machine, now)
	if err != nil {
		securityLog(dataDir, "授权失败", err.Error()+"; machine="+machine)
		return err
	}
	cloudClaims, err := validateCloudLicense(dataDir, card, claims, now)
	if err != nil {
		securityLog(dataDir, "云授权失败", err.Error()+"; machine="+machine+"; serial="+licensing.Serial(claims))
		return err
	}
	mode := "本机签名授权"
	var cloudLeaseExpiresAt any
	if cloudClaims != nil {
		mode = "云端签名授权"
		cloudLeaseExpiresAt = time.Unix(cloudClaims.ExpiresAt, 0).Format(time.RFC3339)
	}
	status, _ := json.MarshalIndent(map[string]any{
		"authorized": true, "mode": mode, "machine": claims.Machine, "expires_at": time.Unix(claims.ExpiresAt, 0).Format(time.RFC3339),
		"serial": licensing.Serial(claims), "cloud_lease_expires_at": cloudLeaseExpiresAt,
	}, "", "  ")
	_ = os.WriteFile(filepath.Join(dataDir, "license.status.json"), append(status, '\n'), 0o600)
	return nil
}

func writeLicenseHelp(dataDir, machine string) error {
	if machine == "" {
		return errors.New("机器码生成失败")
	}
	if err := os.WriteFile(filepath.Join(dataDir, "机器码.txt"), []byte(machine+"\r\n"), 0o600); err != nil {
		return err
	}
	help := "仙尘授权说明\r\n\r\n机器码: " + machine + "\r\n\r\n将机器码发给作者，作者使用独立卡密生成器签发。\r\n收到卡密后，点击插件的设置按钮，在授权窗口粘贴完整卡密并激活。\r\n设置窗口会自动保存授权，无需手动创建或编辑license.key。\r\n卡密与机器码绑定且带到期时间，修改卡密内容会导致签名验证失败。\r\n"
	return os.WriteFile(filepath.Join(dataDir, "授权说明.txt"), []byte(help), 0o600)
}

func securityLog(dataDir, event, detail string) {
	line := fmt.Sprintf("%s [%s] %s\r\n", time.Now().Format("2006-01-02 15:04:05"), event, detail)
	file, err := os.OpenFile(filepath.Join(dataDir, "security.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		_, _ = file.WriteString(line)
		_ = file.Close()
	}
}
