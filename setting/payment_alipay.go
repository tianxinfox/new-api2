package setting

const (
	AlipayPayModePage      = "page"
	AlipayPayModePreCreate = "precreate"
)

var AlipayEnabled = false
var AlipayAppID = ""
var AlipayPrivateKey = ""
var AlipayPublicKey = ""
var AlipayUseCertificateMode = false
var AlipayAppPublicCert = ""
var AlipayAlipayPublicCert = ""
var AlipayRootCert = ""
var AlipaySandbox = false
var AlipayPayMode = AlipayPayModePage
var AlipayOrderExpireMinutes = 30
var AlipayPendingSweepDelayMinutes = 30
