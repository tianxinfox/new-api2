package controller

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/shopspring/decimal"
	alipay "github.com/smartwalle/alipay/v3"
)

type alipayClientCache struct {
	mu         sync.RWMutex
	appID      string
	privateKey string
	publicKey  string
	useCert    bool
	appCert    string
	aliCert    string
	rootCert   string
	sandbox    bool
	client     *alipay.Client
}

var cachedAlipayClient alipayClientCache

func IsAlipayConfigured() bool {
	if !setting.AlipayEnabled {
		return false
	}
	appID := strings.TrimSpace(setting.AlipayAppID)
	privateKey := strings.TrimSpace(setting.AlipayPrivateKey)
	if appID == "" || privateKey == "" {
		return false
	}
	if setting.AlipayUseCertificateMode {
		return strings.TrimSpace(setting.AlipayAppPublicCert) != "" &&
			strings.TrimSpace(setting.AlipayAlipayPublicCert) != "" &&
			strings.TrimSpace(setting.AlipayRootCert) != ""
	}
	return strings.TrimSpace(setting.AlipayPublicKey) != ""
}

func getAlipayClient() (*alipay.Client, error) {
	appID := strings.TrimSpace(setting.AlipayAppID)
	privateKey := strings.TrimSpace(setting.AlipayPrivateKey)
	publicKey := strings.TrimSpace(setting.AlipayPublicKey)
	useCert := setting.AlipayUseCertificateMode
	appCert := strings.TrimSpace(setting.AlipayAppPublicCert)
	aliCert := strings.TrimSpace(setting.AlipayAlipayPublicCert)
	rootCert := strings.TrimSpace(setting.AlipayRootCert)
	sandbox := setting.AlipaySandbox

	if appID == "" || privateKey == "" {
		return nil, fmt.Errorf("alipay config is incomplete")
	}
	if useCert {
		if appCert == "" || aliCert == "" || rootCert == "" {
			return nil, fmt.Errorf("alipay certificate config is incomplete")
		}
	} else if publicKey == "" {
		return nil, fmt.Errorf("alipay public key config is incomplete")
	}

	cachedAlipayClient.mu.RLock()
	if cachedAlipayClient.client != nil &&
		cachedAlipayClient.appID == appID &&
		cachedAlipayClient.privateKey == privateKey &&
		cachedAlipayClient.publicKey == publicKey &&
		cachedAlipayClient.useCert == useCert &&
		cachedAlipayClient.appCert == appCert &&
		cachedAlipayClient.aliCert == aliCert &&
		cachedAlipayClient.rootCert == rootCert &&
		cachedAlipayClient.sandbox == sandbox {
		client := cachedAlipayClient.client
		cachedAlipayClient.mu.RUnlock()
		return client, nil
	}
	cachedAlipayClient.mu.RUnlock()

	// Double-check under write lock to avoid duplicate client construction
	// when multiple goroutines miss the read path concurrently.
	cachedAlipayClient.mu.Lock()
	defer cachedAlipayClient.mu.Unlock()
	if cachedAlipayClient.client != nil &&
		cachedAlipayClient.appID == appID &&
		cachedAlipayClient.privateKey == privateKey &&
		cachedAlipayClient.publicKey == publicKey &&
		cachedAlipayClient.useCert == useCert &&
		cachedAlipayClient.appCert == appCert &&
		cachedAlipayClient.aliCert == aliCert &&
		cachedAlipayClient.rootCert == rootCert &&
		cachedAlipayClient.sandbox == sandbox {
		return cachedAlipayClient.client, nil
	}

	client, err := alipay.New(appID, privateKey, !sandbox)
	if err != nil {
		return nil, err
	}
	if useCert {
		if err = client.LoadAppPublicCert(appCert); err != nil {
			return nil, err
		}
		if err = client.LoadAliPayPublicCert(aliCert); err != nil {
			return nil, err
		}
		if err = client.LoadAliPayRootCert(rootCert); err != nil {
			return nil, err
		}
	} else {
		if err = client.LoadAliPayPublicKey(publicKey); err != nil {
			return nil, err
		}
	}

	cachedAlipayClient.appID = appID
	cachedAlipayClient.privateKey = privateKey
	cachedAlipayClient.publicKey = publicKey
	cachedAlipayClient.useCert = useCert
	cachedAlipayClient.appCert = appCert
	cachedAlipayClient.aliCert = aliCert
	cachedAlipayClient.rootCert = rootCert
	cachedAlipayClient.sandbox = sandbox
	cachedAlipayClient.client = client
	return client, nil
}

func parseAlipayNotification(ctx context.Context, values url.Values) (*alipay.Notification, error) {
	client, err := getAlipayClient()
	if err != nil {
		return nil, err
	}
	if err = client.VerifySign(ctx, values); err != nil {
		return nil, err
	}
	return client.DecodeNotification(ctx, values)
}

func alipayMoneyToCents(money float64) int64 {
	return decimal.NewFromFloat(money).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
}

func alipayMoneyStrToCents(money string) (int64, error) {
	d, err := decimal.NewFromString(strings.TrimSpace(money))
	if err != nil {
		return 0, err
	}
	return d.Mul(decimal.NewFromInt(100)).Round(0).IntPart(), nil
}

func getAlipayPayMode() string {
	mode := strings.ToLower(strings.TrimSpace(setting.AlipayPayMode))
	if mode == setting.AlipayPayModePreCreate {
		return setting.AlipayPayModePreCreate
	}
	return setting.AlipayPayModePage
}

func getAlipayProductCode(payMode string) string {
	if payMode == setting.AlipayPayModePreCreate {
		return "FACE_TO_FACE_PAYMENT"
	}
	return "FAST_INSTANT_TRADE_PAY"
}

func logAlipayPreCreateError(prefix string, userID int, tradeNo string, err error, rsp *alipay.TradePreCreateRsp) {
	code, msg, subCode, subMsg := "", "", "", ""
	qrCodeEmpty := false
	if rsp != nil {
		code = string(rsp.Code)
		msg = rsp.Msg
		subCode = rsp.SubCode
		subMsg = rsp.SubMsg
		qrCodeEmpty = rsp.QRCode == ""
	}
	common.SysError(fmt.Sprintf("%s: user_id=%d trade_no=%s err=%v rsp_nil=%t code=%s msg=%s sub_code=%s sub_msg=%s qr_code_empty=%t",
		prefix, userID, tradeNo, err, rsp == nil, code, msg, subCode, subMsg, qrCodeEmpty))
}

func getAlipayPreCreateFailureMessage(err error, rsp *alipay.TradePreCreateRsp) string {
	if err != nil {
		return fmt.Sprintf("alipay precreate failed: %v", err)
	}
	if rsp == nil {
		return "alipay precreate failed: empty response"
	}
	if rsp.SubMsg != "" {
		return fmt.Sprintf("alipay precreate failed: [%s] %s", rsp.SubCode, rsp.SubMsg)
	}
	if rsp.Msg != "" {
		return fmt.Sprintf("alipay precreate failed: [%s] %s", rsp.Code, rsp.Msg)
	}
	return "alipay precreate failed"
}
