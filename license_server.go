package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xianlv/internal/licensing"
)

var licenseServerState struct {
	sync.Mutex
	server  *http.Server
	url     string
	dataDir string
}

func startLicenseServer(dataDir string) (string, error) {
	licenseServerState.Lock()
	defer licenseServerState.Unlock()
	if licenseServerState.server != nil && strings.EqualFold(licenseServerState.dataDir, dataDir) {
		return licenseServerState.url, nil
	}
	if licenseServerState.server != nil {
		_ = licenseServerState.server.Close()
		licenseServerState.server = nil
	}
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	var listener net.Listener
	var err error
	for port := 8099; port <= 8109; port++ {
		listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			break
		}
	}
	if listener == nil {
		return "", fmt.Errorf("授权设置端口不可用: %w", err)
	}
	handler := newLicenseActivationHandler(dataDir, token, func() (string, error) {
		if err := ensureRuntimeDataDir(dataDir); err != nil {
			return "", err
		}
		return currentAdminURL(), nil
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second}
	licenseServerState.server = server
	licenseServerState.dataDir = dataDir
	licenseServerState.url = "http://" + listener.Addr().String() + "/activate"
	go func() { _ = server.Serve(listener) }()
	return licenseServerState.url, nil
}

func stopLicenseServer() {
	licenseServerState.Lock()
	server := licenseServerState.server
	licenseServerState.server = nil
	licenseServerState.url = ""
	licenseServerState.dataDir = ""
	licenseServerState.Unlock()
	if server != nil {
		_ = server.Close()
	}
}

func newLicenseActivationHandler(dataDir, token string, activate func() (string, error)) http.Handler {
	page := template.Must(template.New("activation").Parse(licenseActivationHTML))
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		response.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'self'")
		if request.URL.Path != "/activate" {
			http.NotFound(response, request)
			return
		}
		switch request.Method {
		case http.MethodGet:
			response.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = page.Execute(response, map[string]string{"Machine": licensing.MachineCode(), "Token": token})
		case http.MethodPost:
			if request.Header.Get("X-License-Token") != token {
				writeActivationJSON(response, http.StatusForbidden, false, "授权页面令牌无效，请重新点击插件设置", "")
				return
			}
			request.Body = http.MaxBytesReader(response, request.Body, 64*1024)
			var input struct {
				License string `json:"license"`
			}
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				writeActivationJSON(response, http.StatusBadRequest, false, "卡密内容无法读取", "")
				return
			}
			card := strings.TrimSpace(input.License)
			claims, err := licensing.Verify(card, licensing.MachineCode(), time.Now())
			if err != nil {
				securityLog(dataDir, "设置窗口授权失败", err.Error())
				writeActivationJSON(response, http.StatusBadRequest, false, err.Error(), "")
				return
			}
			if err := os.MkdirAll(dataDir, 0o755); err != nil {
				writeActivationJSON(response, http.StatusInternalServerError, false, err.Error(), "")
				return
			}
			if err := os.WriteFile(filepath.Join(dataDir, licenseFilename), []byte(card+"\r\n"), 0o600); err != nil {
				writeActivationJSON(response, http.StatusInternalServerError, false, "保存授权失败: "+err.Error(), "")
				return
			}
			adminURL, err := activate()
			if err != nil {
				writeActivationJSON(response, http.StatusInternalServerError, false, "授权有效，但后台启动失败: "+err.Error(), "")
				return
			}
			securityLog(dataDir, "设置窗口授权成功", "expires="+time.Unix(claims.ExpiresAt, 0).Format(time.RFC3339))
			writeActivationJSON(response, http.StatusOK, true, "授权成功，正在进入数据后台", adminURL)
		default:
			response.Header().Set("Allow", "GET, POST")
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func writeActivationJSON(response http.ResponseWriter, status int, ok bool, message, adminURL string) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]any{"ok": ok, "message": message, "admin_url": adminURL})
}

const licenseActivationHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>仙尘 · 插件授权</title>
<style>
:root{color-scheme:light;font-family:"Microsoft YaHei UI","PingFang SC",sans-serif;color:#202421;background:#eef2ef}*{box-sizing:border-box;letter-spacing:0}body{margin:0;min-height:100vh;display:grid;place-items:center;padding:24px;background:#eef2ef}.shell{width:min(720px,100%);overflow:hidden;background:#fff;border:1px solid #d7dfd9;border-top:3px solid #b58a3a;border-radius:7px;box-shadow:0 22px 54px rgba(24,34,29,.16)}header{display:flex;align-items:center;gap:14px;padding:22px 26px;border-bottom:1px solid #e3e8e4;background:#fbfcfb}.mark{width:46px;height:46px;display:grid;place-items:center;border-radius:6px;border:1px solid #29473b;background:#18251f;color:#fff;font-size:19px;font-weight:700;box-shadow:0 5px 14px rgba(24,37,31,.18)}h1{margin:0;font-size:21px}header p{margin:4px 0 0;color:#69736d;font-size:12px}.edition{margin-left:auto;padding:5px 8px;border:1px solid #eadfc7;border-radius:4px;background:#f8f3e8;color:#7c612e;font-size:11px;font-weight:700}main{padding:25px 26px}.trust{display:grid;grid-template-columns:repeat(3,1fr);margin-bottom:22px;border:1px solid #dfe5e1;border-radius:5px;background:#f7f9f7}.trust span{padding:10px 12px;border-right:1px solid #dfe5e1;color:#526058;font-size:11px;text-align:center}.trust span:last-child{border-right:0}.trust b{display:inline-block;width:7px;height:7px;margin-right:6px;border-radius:50%;background:#1f9a68}.label{display:block;margin-bottom:7px;color:#435048;font-size:12px;font-weight:700}.machine{display:flex;gap:8px;margin-bottom:20px}.machine code{min-width:0;flex:1;padding:11px 12px;border:1px solid #d5dcd7;border-radius:4px;background:#f7f9f7;color:#26342d;overflow-wrap:anywhere}button,textarea{font:inherit;letter-spacing:0}button{min-height:40px;padding:0 16px;border:1px solid #ccd5cf;border-radius:4px;background:#fff;color:#202421;cursor:pointer;transition:.14s ease}button:hover{border-color:#9eaaa2;background:#f7f9f7}button.primary{width:100%;margin-top:14px;border-color:#0f6247;background:#167052;color:#fff;font-weight:700;box-shadow:0 6px 16px rgba(22,112,82,.18)}button.primary:hover{background:#125f46}button:disabled{opacity:.55;cursor:default}textarea{width:100%;min-height:126px;padding:13px;border:1px solid #c5cec8;border-radius:5px;background:#fbfcfb;outline:0;resize:vertical;word-break:break-all;line-height:1.55}textarea:focus{border-color:#167052;box-shadow:0 0 0 3px rgba(22,112,82,.12)}.hint{margin:7px 0 0;color:#7a847e;font-size:11px}.status{min-height:23px;margin:12px 0 0;color:#66706a;font-size:13px}.status.error{color:#a93a32}.status.success{color:#167052}footer{display:flex;justify-content:space-between;gap:20px;padding:15px 26px;border-top:1px solid #e3e8e4;background:#fbfcfb;color:#758079;font-size:11px}footer strong{color:#536159;font-weight:650}@media(max-width:560px){body{display:block;padding:0;background:#fff}.shell{min-height:100vh;border:0;border-top:3px solid #b58a3a;border-radius:0;box-shadow:none}header,main,footer{padding-left:18px;padding-right:18px}.edition{display:none}.trust{grid-template-columns:1fr}.trust span{border-right:0;border-bottom:1px solid #dfe5e1}.trust span:last-child{border-bottom:0}.machine{align-items:stretch;flex-direction:column}.machine button{width:100%}footer{flex-direction:column;gap:4px}}
</style>
</head>
<body>
<section class="shell">
<header><div class="mark">尘</div><div><h1>仙尘插件授权</h1><p>作者：随缘 · 本机安全验证</p></div><span class="edition">XC2 授权中心</span></header>
<main>
<div class="trust"><span><b></b>机器绑定</span><span><b></b>签名防伪</span><span><b></b>到期校验</span></div>
<span class="label">本机机器码</span>
<div class="machine"><code id="machine">{{.Machine}}</code><button id="copyButton" type="button">复制机器码</button></div>
<label class="label" for="license">授权卡密</label>
<textarea id="license" autocomplete="off" spellcheck="false" placeholder="粘贴作者签发的XC2紧凑卡密或旧版签名卡密"></textarea>
<p class="hint">卡密会在本机完成签名、机器码与有效期校验，不会上传玩家数据库。</p>
<button class="primary" id="activateButton" type="button">验证并激活</button>
<p class="status" id="status" role="status"></p>
</main>
<footer><strong>仙尘 · 本机授权</strong><span>卡密仅保存在 plugin_data/仙尘，不写入玩家数据表。</span></footer>
</section>
<script>
const token={{.Token}};const status=document.getElementById('status');const button=document.getElementById('activateButton');
document.getElementById('copyButton').addEventListener('click',async()=>{await navigator.clipboard.writeText(document.getElementById('machine').textContent);status.className='status success';status.textContent='机器码已复制';});
button.addEventListener('click',async()=>{const license=document.getElementById('license').value.trim();if(!license){status.className='status error';status.textContent='请先粘贴完整卡密';return}button.disabled=true;button.textContent='正在验证';status.className='status';status.textContent='';try{const response=await fetch('/activate',{method:'POST',headers:{'Content-Type':'application/json','X-License-Token':token},body:JSON.stringify({license})});const data=await response.json();if(!response.ok||!data.ok)throw new Error(data.message||'授权失败');status.className='status success';status.textContent=data.message;button.textContent='授权成功';setTimeout(()=>location.assign(data.admin_url),500)}catch(error){status.className='status error';status.textContent=error.message;button.disabled=false;button.textContent='验证并激活'}});
</script>
</body>
</html>`
