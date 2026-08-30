package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xianlv/internal/appinfo"
	"xianlv/internal/licensing"
)

const cloudLeaseFilename = "license.cloud.lease"

var cloudLicenseState struct {
	sync.Mutex
	identity  string
	lease     licensing.CloudLeaseClaims
	nextCheck time.Time
}

type cloudLicenseRequest struct {
	Product          string `json:"product"`
	Version          string `json:"version"`
	Machine          string `json:"machine"`
	Serial           string `json:"serial"`
	License          string `json:"license"`
	LicenseExpiresAt int64  `json:"license_expires_at"`
	Challenge        string `json:"challenge"`
}

type cloudLicenseResponse struct {
	Lease   string `json:"lease"`
	Message string `json:"message"`
}

func validateCloudLicense(dataDir, card string, local licensing.Claims, now time.Time) (*licensing.CloudLeaseClaims, error) {
	endpoint := strings.TrimSpace(os.Getenv("XIANLV_LICENSE_CLOUD_URL"))
	if endpoint == "" {
		return nil, nil
	}
	if err := validateCloudLicenseURL(endpoint); err != nil {
		return nil, err
	}
	identity := endpoint + "|" + strings.ToUpper(strings.TrimSpace(local.Machine)) + "|" + licensing.Serial(local)
	cloudLicenseState.Lock()
	defer cloudLicenseState.Unlock()
	if cloudLicenseState.identity == identity && now.Before(cloudLicenseState.nextCheck) && cloudLicenseState.lease.ExpiresAt > now.Unix() {
		lease := cloudLicenseState.lease
		return &lease, nil
	}
	challengeBytes := make([]byte, 24)
	if _, err := rand.Read(challengeBytes); err != nil {
		return nil, err
	}
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes)
	request := cloudLicenseRequest{
		Product: licensing.ProductID, Version: appinfo.Version, Machine: local.Machine, Serial: licensing.Serial(local),
		License: strings.TrimSpace(card), LicenseExpiresAt: local.ExpiresAt, Challenge: challenge,
	}
	lease, authoritative, err := requestCloudLicense(endpoint, request, now)
	if err == nil {
		if lease.Challenge != challenge {
			return nil, errors.New("云授权响应与本次校验请求不匹配")
		}
		if err := writeCloudLease(filepath.Join(dataDir, cloudLeaseFilename), lease.raw); err != nil {
			return nil, fmt.Errorf("保存云授权租约: %w", err)
		}
		rememberCloudLicense(identity, lease.CloudLeaseClaims, now.Add(10*time.Minute))
		return &lease.CloudLeaseClaims, nil
	}
	if authoritative {
		_ = os.Remove(filepath.Join(dataDir, cloudLeaseFilename))
		clearCloudLicense(identity)
		return nil, err
	}
	cached, cacheErr := os.ReadFile(filepath.Join(dataDir, cloudLeaseFilename))
	if cacheErr != nil {
		return nil, fmt.Errorf("云授权暂时不可用且没有有效离线租约: %w", err)
	}
	claims, cacheErr := licensing.VerifyCloudLease(strings.TrimSpace(string(cached)), local.Machine, licensing.Serial(local), local.ExpiresAt, now)
	if cacheErr != nil {
		return nil, fmt.Errorf("云授权暂时不可用，离线租约也无效: %w", cacheErr)
	}
	rememberCloudLicense(identity, claims, now.Add(2*time.Minute))
	return &claims, nil
}

func rememberCloudLicense(identity string, lease licensing.CloudLeaseClaims, requestedCheck time.Time) {
	expiresAt := time.Unix(lease.ExpiresAt, 0)
	if requestedCheck.After(expiresAt) {
		requestedCheck = expiresAt
	}
	cloudLicenseState.identity = identity
	cloudLicenseState.lease = lease
	cloudLicenseState.nextCheck = requestedCheck
}

func clearCloudLicense(identity string) {
	if cloudLicenseState.identity != identity {
		return
	}
	cloudLicenseState.identity = ""
	cloudLicenseState.lease = licensing.CloudLeaseClaims{}
	cloudLicenseState.nextCheck = time.Time{}
}

type verifiedCloudLease struct {
	licensing.CloudLeaseClaims
	raw string
}

func requestCloudLicense(endpoint string, input cloudLicenseRequest, now time.Time) (verifiedCloudLease, bool, error) {
	var verified verifiedCloudLease
	payload, err := json.Marshal(input)
	if err != nil {
		return verified, false, err
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return verified, false, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Xianchen/"+appinfo.Version)
	if token := strings.TrimSpace(os.Getenv("XIANLV_LICENSE_CLOUD_TOKEN")); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := (&http.Client{Timeout: 8 * time.Second}).Do(request)
	if err != nil {
		return verified, false, err
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if readErr != nil {
		return verified, false, readErr
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		var cloudResponse cloudLicenseResponse
		if json.Unmarshal(body, &cloudResponse) == nil && strings.TrimSpace(cloudResponse.Message) != "" {
			message = strings.TrimSpace(cloudResponse.Message)
		}
		if message == "" {
			message = response.Status
		}
		transient := response.StatusCode == http.StatusRequestTimeout || response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
		return verified, !transient, fmt.Errorf("云授权拒绝: %s", message)
	}
	var output cloudLicenseResponse
	if err := json.Unmarshal(body, &output); err != nil || strings.TrimSpace(output.Lease) == "" {
		return verified, true, errors.New("云授权响应缺少有效租约")
	}
	claims, err := licensing.VerifyCloudLease(output.Lease, input.Machine, input.Serial, input.LicenseExpiresAt, now)
	if err != nil {
		return verified, true, err
	}
	verified.CloudLeaseClaims = claims
	verified.raw = strings.TrimSpace(output.Lease)
	return verified, false, nil
}

func validateCloudLicenseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return errors.New("云授权地址格式不正确")
	}
	host := parsed.Hostname()
	loopback := strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
	if !strings.EqualFold(parsed.Scheme, "https") && !(strings.EqualFold(parsed.Scheme, "http") && loopback) {
		return errors.New("云授权地址必须使用HTTPS")
	}
	return nil
}

func writeCloudLease(path, lease string) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(strings.TrimSpace(lease)+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}
