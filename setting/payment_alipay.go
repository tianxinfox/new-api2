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
var AgentWithdrawEnabled = false
var AgentWithdrawMinAmount = 1.0
var AgentWithdrawOrderTitle = "代理佣金提现"
var AgentWithdrawSceneName = "佣金报酬"
var AgentWithdrawTransferSceneReportInfos = ""
