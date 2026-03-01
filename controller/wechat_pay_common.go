package controller

import (
	"context"
	"crypto/rsa"
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

const PaymentMethodWeChat = "wechat"
const CNYToCentsMultiplier int64 = 100
const defaultWeChatNativeExpireMinutes = 5
const defaultWeChatDelayedCheckMinutes = 6
const defaultWeChatPendingSweepHours = 24

var weChatPrivateKeyCache struct {
	mu     sync.RWMutex
	rawKey string
	parsed *rsa.PrivateKey
}

func IsWeChatPayConfigured() bool {
	if !setting.WeChatPayEnabled {
		return false
	}
	return setting.WeChatPayMchID != "" &&
		setting.WeChatPayAppID != "" &&
		setting.WeChatPayAPIv3Key != "" &&
		setting.WeChatPayMchSerial != "" &&
		setting.WeChatPayPrivateKey != ""
}

func getWeChatPrivateKey() (*rsa.PrivateKey, error) {
	key := strings.TrimSpace(setting.WeChatPayPrivateKey)
	if key == "" {
		return nil, fmt.Errorf("wechat private key is empty")
	}
	key = strings.ReplaceAll(key, `\n`, "\n")

	weChatPrivateKeyCache.mu.RLock()
	if weChatPrivateKeyCache.rawKey == key && weChatPrivateKeyCache.parsed != nil {
		parsed := weChatPrivateKeyCache.parsed
		weChatPrivateKeyCache.mu.RUnlock()
		return parsed, nil
	}
	weChatPrivateKeyCache.mu.RUnlock()

	privateKey, err := utils.LoadPrivateKey(key)
	if err != nil {
		return nil, err
	}

	weChatPrivateKeyCache.mu.Lock()
	weChatPrivateKeyCache.rawKey = key
	weChatPrivateKeyCache.parsed = privateKey
	weChatPrivateKeyCache.mu.Unlock()

	return privateKey, nil
}

func getWeChatPayClient(ctx context.Context) (*core.Client, error) {
	privateKey, err := getWeChatPrivateKey()
	if err != nil {
		return nil, err
	}
	return core.NewClient(ctx,
		option.WithWechatPayAutoAuthCipher(
			setting.WeChatPayMchID,
			setting.WeChatPayMchSerial,
			privateKey,
			setting.WeChatPayAPIv3Key,
		),
	)
}

func getWeChatNotifyHandler(ctx context.Context) (*notify.Handler, error) {
	privateKey, err := getWeChatPrivateKey()
	if err != nil {
		return nil, err
	}
	if err = downloader.MgrInstance().RegisterDownloaderWithPrivateKey(
		ctx,
		privateKey,
		setting.WeChatPayMchSerial,
		setting.WeChatPayMchID,
		setting.WeChatPayAPIv3Key,
	); err != nil {
		return nil, err
	}
	certificateVisitor := downloader.MgrInstance().GetCertificateVisitor(setting.WeChatPayMchID)
	return notify.NewNotifyHandler(
		setting.WeChatPayAPIv3Key,
		verifiers.NewSHA256WithRSAVerifier(certificateVisitor),
	), nil
}

func weChatMoneyToCents(money float64) int64 {
	return decimal.NewFromFloat(money).
		Mul(decimal.NewFromInt(CNYToCentsMultiplier)).
		Round(0).
		IntPart()
}

func getWeChatNativeExpireMinutes() int {
	expireMinutes := common.GetEnvOrDefault("WECHAT_NATIVE_EXPIRE_MINUTES", defaultWeChatNativeExpireMinutes)
	if expireMinutes <= 0 {
		return defaultWeChatNativeExpireMinutes
	}
	return expireMinutes
}

func getWeChatDelayedCheckMinutes() int {
	delayedMinutes := common.GetEnvOrDefault("WECHAT_DELAYED_CHECK_MINUTES", defaultWeChatDelayedCheckMinutes)
	if delayedMinutes <= 0 {
		return defaultWeChatDelayedCheckMinutes
	}
	return delayedMinutes
}

func getWeChatPendingSweepHours() int {
	sweepHours := common.GetEnvOrDefault("WECHAT_PENDING_SWEEP_HOURS", defaultWeChatPendingSweepHours)
	if sweepHours <= 0 {
		return defaultWeChatPendingSweepHours
	}
	return sweepHours
}

// buildWeChatOutTradeNo builds a compact merchant order number for WeChat Pay.
// WeChat v3 limits out_trade_no to <= 32 bytes.
func buildWeChatOutTradeNo(prefix string, userId int, now time.Time) string {
	randomPart := strings.ToUpper(common.GetRandomString(6))
	tradeNo := fmt.Sprintf("%s%d%d%s", prefix, userId, now.Unix(), randomPart)
	if len(tradeNo) <= 32 {
		return tradeNo
	}

	// Keep suffix entropy and unix seconds, and add a short user hash to
	// reduce cross-user collision risk when user id has to be truncated.
	userPart := strconv.Itoa(userId)
	userHash := getCompactUserHash(userId)
	maxUserPartLen := 32 - len(prefix) - len(userHash) - 10 - len(randomPart)
	if maxUserPartLen < 0 {
		maxUserPartLen = 0
	}
	if len(userPart) > maxUserPartLen {
		userPart = userPart[len(userPart)-maxUserPartLen:]
	}
	return fmt.Sprintf("%s%s%s%d%s", prefix, userHash, userPart, now.Unix(), randomPart)
}

func getCompactUserHash(userId int) string {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(strconv.Itoa(userId)))
	const hashBase uint64 = 36 * 36 * 36 * 36 // 4 chars base36
	v := uint64(hasher.Sum32()) % hashBase
	hash := strings.ToUpper(strconv.FormatUint(v, 36))
	if len(hash) >= 4 {
		return hash[len(hash)-4:]
	}
	return strings.Repeat("0", 4-len(hash)) + hash
}

func writeWeChatNotifySuccess(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    "SUCCESS",
		"message": "success",
	})
}

func writeWeChatNotifyFail(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"code":    "FAIL",
		"message": message,
	})
}

func parseWeChatSuccessTransaction(c *gin.Context) (*payments.Transaction, bool) {
	if !IsWeChatPayConfigured() {
		writeWeChatNotifyFail(c, "wechat pay not configured")
		return nil, false
	}
	handler, err := getWeChatNotifyHandler(c.Request.Context())
	if err != nil {
		writeWeChatNotifyFail(c, "notify handler init failed")
		return nil, false
	}

	transaction := new(payments.Transaction)
	_, err = handler.ParseNotifyRequest(c.Request.Context(), c.Request, transaction)
	if err != nil {
		writeWeChatNotifyFail(c, "invalid notify signature")
		return nil, false
	}
	if transaction.OutTradeNo == nil || *transaction.OutTradeNo == "" {
		writeWeChatNotifyFail(c, "out trade no is empty")
		return nil, false
	}
	if transaction.TradeState == nil || *transaction.TradeState != "SUCCESS" {
		writeWeChatNotifySuccess(c)
		return nil, false
	}
	if transaction.Mchid == nil || *transaction.Mchid != setting.WeChatPayMchID {
		writeWeChatNotifyFail(c, "invalid mchid")
		return nil, false
	}
	if transaction.Appid == nil || *transaction.Appid != setting.WeChatPayAppID {
		writeWeChatNotifyFail(c, "invalid appid")
		return nil, false
	}
	if transaction.Amount == nil || transaction.Amount.Total == nil || *transaction.Amount.Total <= 0 {
		writeWeChatNotifyFail(c, "invalid amount")
		return nil, false
	}
	if transaction.Amount.Currency != nil && *transaction.Amount.Currency != "CNY" {
		writeWeChatNotifyFail(c, "invalid currency")
		return nil, false
	}
	return transaction, true
}
