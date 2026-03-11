package model

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

const UserNameMaxLength = 20

// User if you add sensitive fields, don't forget to clean them in setupLogin function.
// Otherwise, the sensitive information will be saved on local storage in plain text!
type User struct {
	Id                   int            `json:"id"`
	CreatedAt            int64          `json:"created_at" gorm:"bigint;autoCreateTime"`
	Username             string         `json:"username" gorm:"unique;index" validate:"max=20"`
	Password             string         `json:"password" gorm:"not null;" validate:"min=8,max=20"`
	OriginalPassword     string         `json:"original_password" gorm:"-:all"` // this field is only for Password change verification, don't save it to database!
	DisplayName          string         `json:"display_name" gorm:"index" validate:"max=20"`
	WithdrawPayeeAccount string         `json:"withdraw_payee_account" gorm:"type:varchar(128);default:''" validate:"max=128"`
	WithdrawPayeeName    string         `json:"withdraw_payee_name" gorm:"type:varchar(64);default:''" validate:"max=64"`
	Role                 int            `json:"role" gorm:"type:int;default:1"`   // admin, common
	Status               int            `json:"status" gorm:"type:int;default:1"` // enabled, disabled
	Email                string         `json:"email" gorm:"index" validate:"max=50"`
	GitHubId             string         `json:"github_id" gorm:"column:github_id;index"`
	DiscordId            string         `json:"discord_id" gorm:"column:discord_id;index"`
	OidcId               string         `json:"oidc_id" gorm:"column:oidc_id;index"`
	WeChatId             string         `json:"wechat_id" gorm:"column:wechat_id;index"`
	TelegramId           string         `json:"telegram_id" gorm:"column:telegram_id;index"`
	VerificationCode     string         `json:"verification_code" gorm:"-:all"`                                    // this field is only for Email verification, don't save it to database!
	AccessToken          *string        `json:"access_token" gorm:"type:char(32);column:access_token;uniqueIndex"` // this token is for system management
	Quota                int            `json:"quota" gorm:"type:int;default:0"`
	UsedQuota            int            `json:"used_quota" gorm:"type:int;default:0;column:used_quota"` // used quota
	RequestCount         int            `json:"request_count" gorm:"type:int;default:0;"`               // request number
	Group                string         `json:"group" gorm:"type:varchar(64);default:'default'"`
	AffCode              string         `json:"aff_code" gorm:"type:varchar(32);column:aff_code;uniqueIndex"`
	AffCount             int            `json:"aff_count" gorm:"type:int;default:0;column:aff_count"`
	AffQuota             int            `json:"aff_quota" gorm:"type:int;default:0;column:aff_quota"`           // 邀请剩余额度
	AffHistoryQuota      int            `json:"aff_history_quota" gorm:"type:int;default:0;column:aff_history"` // 邀请历史额度
	InviterId            int            `json:"inviter_id" gorm:"type:int;column:inviter_id;index"`
	RebateRate           int            `json:"rebate_rate" gorm:"type:int;default:0;column:rebate_rate"` // agent rebate rate in bps, 10000 = 100%
	DeletedAt            gorm.DeletedAt `gorm:"index"`
	LinuxDOId            string         `json:"linux_do_id" gorm:"column:linux_do_id;index"`
	Setting              string         `json:"setting" gorm:"type:text;column:setting"`
	Remark               string         `json:"remark,omitempty" gorm:"type:varchar(255)" validate:"max=255"`
	StripeCustomer       string         `json:"stripe_customer" gorm:"type:varchar(64);column:stripe_customer;index"`
}

func (user *User) ToBaseUser() *UserBase {
	cache := &UserBase{
		Id:       user.Id,
		Group:    user.Group,
		Quota:    user.Quota,
		Status:   user.Status,
		Username: user.Username,
		Setting:  user.Setting,
		Email:    user.Email,
	}
	return cache
}

func (user *User) GetAccessToken() string {
	if user.AccessToken == nil {
		return ""
	}
	return *user.AccessToken
}

func (user *User) SetAccessToken(token string) {
	user.AccessToken = &token
}

func (user *User) GetSetting() dto.UserSetting {
	setting := dto.UserSetting{}
	if user.Setting != "" {
		err := common.UnmarshalJsonStr(user.Setting, &setting)
		if err != nil {
			common.SysLog("failed to unmarshal setting: " + err.Error())
		}
	}
	return setting
}

func (user *User) SetSetting(setting dto.UserSetting) {
	settingBytes, err := common.Marshal(setting)
	if err != nil {
		common.SysLog("failed to marshal setting: " + err.Error())
		return
	}
	user.Setting = string(settingBytes)
}

// 根据用户角色生成默认的边栏配置
func generateDefaultSidebarConfigForRole(userRole int) string {
	defaultConfig := map[string]interface{}{}

	// 聊天区域 - 所有用户都可以访问
	defaultConfig["chat"] = map[string]interface{}{
		"enabled":    true,
		"playground": true,
		"chat":       true,
	}

	// 控制台区域 - 所有用户都可以访问
	defaultConfig["console"] = map[string]interface{}{
		"enabled":    true,
		"detail":     true,
		"token":      true,
		"log":        true,
		"midjourney": true,
		"task":       true,
	}

	// 个人中心区域 - 所有用户都可以访问
	defaultConfig["personal"] = map[string]interface{}{
		"enabled":  true,
		"topup":    true,
		"personal": true,
	}

	// 代理区域 - 代理及以上角色
	if userRole >= common.RoleAgentUser {
		defaultConfig["agent"] = map[string]interface{}{
			"enabled":        true,
			"agentDashboard": true,
			"agentUsers":     true,
			"agentTopups":    true,
			"agentRebates":   true,
		}
	}

	// 管理员区域 - 根据角色决定
	if userRole == common.RoleAdminUser {
		defaultConfig["admin"] = map[string]interface{}{
			"enabled":            true,
			"adminAgentOverview": true,
			"channel":            true,
			"models":             true,
			"redemption":         true,
			"user":               true,
			"setting":            false,
		}
	} else if userRole == common.RoleRootUser {
		defaultConfig["admin"] = map[string]interface{}{
			"enabled":            true,
			"adminAgentOverview": true,
			"channel":            true,
			"models":             true,
			"redemption":         true,
			"user":               true,
			"setting":            true,
		}
	}

	// 转换为JSON字符串
	configBytes, err := common.Marshal(defaultConfig)
	if err != nil {
		common.SysLog("生成默认边栏配置失败: " + err.Error())
		return ""
	}

	return string(configBytes)
}

// CheckUserExistOrDeleted check if user exist or deleted, if not exist, return false, nil, if deleted or exist, return true, nil
func CheckUserExistOrDeleted(username string, email string) (bool, error) {
	var user User

	// err := DB.Unscoped().First(&user, "username = ? or email = ?", username, email).Error
	// check email if empty
	var err error
	if email == "" {
		err = DB.Unscoped().First(&user, "username = ?", username).Error
	} else {
		err = DB.Unscoped().First(&user, "username = ? or email = ?", username, email).Error
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// not exist, return false, nil
			return false, nil
		}
		// other error, return false, err
		return false, err
	}
	// exist, return true, nil
	return true, nil
}

func GetMaxUserId() int {
	var user User
	DB.Unscoped().Last(&user)
	return user.Id
}

func GetAllUsers(pageInfo *common.PageInfo) (users []*User, total int64, err error) {
	// Start transaction
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get total count within transaction
	err = tx.Unscoped().Model(&User{}).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated users within same transaction
	err = tx.Unscoped().Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Omit("password").Find(&users).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Commit transaction
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func SearchUsers(keyword string, group string, startIdx int, num int) ([]*User, int64, error) {
	var users []*User
	var total int64
	var err error

	// 开始事务
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 构建基础查询
	query := tx.Unscoped().Model(&User{})

	// 构建搜索条件
	likeCondition := "username LIKE ? OR email LIKE ? OR display_name LIKE ?"

	// 尝试将关键字转换为整数ID
	keywordInt, err := strconv.Atoi(keyword)
	if err == nil {
		// 如果是数字，同时搜索ID和其他字段
		likeCondition = "id = ? OR " + likeCondition
		if group != "" {
			query = query.Where("("+likeCondition+") AND "+commonGroupCol+" = ?",
				keywordInt, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", group)
		} else {
			query = query.Where(likeCondition,
				keywordInt, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
		}
	} else {
		// 非数字关键字，只搜索字符串字段
		if group != "" {
			query = query.Where("("+likeCondition+") AND "+commonGroupCol+" = ?",
				"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", group)
		} else {
			query = query.Where(likeCondition,
				"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
		}
	}

	// 获取总数
	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 获取分页数据
	err = query.Omit("password").Order("id desc").Limit(num).Offset(startIdx).Find(&users).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func GetUserById(id int, selectAll bool) (*User, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	user := User{Id: id}
	var err error = nil
	if selectAll {
		err = DB.First(&user, "id = ?", id).Error
	} else {
		err = DB.Omit("password").First(&user, "id = ?", id).Error
	}
	return &user, err
}

func GetUserIdByAffCode(affCode string) (int, error) {
	if affCode == "" {
		return 0, errors.New("affCode 为空！")
	}
	var user User
	err := DB.Select("id").First(&user, "aff_code = ?", affCode).Error
	return user.Id, err
}

func DeleteUserById(id int) (err error) {
	if id == 0 {
		return errors.New("id 为空！")
	}
	user := User{Id: id}
	return user.Delete()
}

func HardDeleteUserById(id int) error {
	if id == 0 {
		return errors.New("id 为空！")
	}
	err := DB.Unscoped().Delete(&User{}, "id = ?", id).Error
	return err
}

func inviteUser(inviterId int) (err error) {
	user, err := GetUserById(inviterId, true)
	if err != nil {
		return err
	}
	user.AffCount++
	user.AffQuota += common.QuotaForInviter
	user.AffHistoryQuota += common.QuotaForInviter
	return DB.Save(user).Error
}

func (user *User) TransferAffQuotaToQuota(quota int) error {
	// 检查quota是否小于最小额度
	if float64(quota) < common.QuotaPerUnit {
		return fmt.Errorf("转移额度最小为%s！", logger.LogQuota(int(common.QuotaPerUnit)))
	}

	// 开始数据库事务
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback() // 确保在函数退出时事务能回滚

	// 加锁查询用户以确保数据一致性
	err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, user.Id).Error
	if err != nil {
		return err
	}

	// 再次检查用户的AffQuota是否足够
	if user.AffQuota < quota {
		return errors.New("邀请额度不足！")
	}

	// 更新用户额度
	user.AffQuota -= quota
	user.Quota += quota

	// 保存用户状态
	if err := tx.Save(user).Error; err != nil {
		return err
	}

	// 提交事务
	return tx.Commit().Error
}

func (user *User) Insert(inviterId int) error {
	var err error
	if user.Password != "" {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	user.Quota = common.QuotaForNewUser
	//user.SetAccessToken(common.GetUUID())
	user.AffCode = common.GetRandomString(4)

	// 初始化用户设置，包括默认的边栏配置
	if user.Setting == "" {
		defaultSetting := dto.UserSetting{}
		// 这里暂时不设置SidebarModules，因为需要在用户创建后根据角色设置
		user.SetSetting(defaultSetting)
	}

	result := DB.Create(user)
	if result.Error != nil {
		return result.Error
	}

	// 用户创建成功后，根据角色初始化边栏配置
	defaultSidebarConfig := generateDefaultSidebarConfigForRole(user.Role)
	if defaultSidebarConfig != "" {
		currentSetting := user.GetSetting()
		currentSetting.SidebarModules = defaultSidebarConfig
		user.SetSetting(currentSetting)
		_ = user.Update(false)
		common.SysLog(fmt.Sprintf("为新用户 %s (角色: %d) 初始化边栏配置", user.Username, user.Role))
	}

	if common.QuotaForNewUser > 0 {
		RecordLog(user.Id, LogTypeSystem, fmt.Sprintf("新用户注册赠送 %s", logger.LogQuota(common.QuotaForNewUser)))
	}
	if inviterId != 0 {
		if common.QuotaForInvitee > 0 {
			_ = IncreaseUserQuota(user.Id, common.QuotaForInvitee, true)
			RecordLog(user.Id, LogTypeSystem, fmt.Sprintf("使用邀请码赠送 %s", logger.LogQuota(common.QuotaForInvitee)))
		}
		if common.QuotaForInviter > 0 {
			//_ = IncreaseUserQuota(inviterId, common.QuotaForInviter)
			RecordLog(inviterId, LogTypeSystem, fmt.Sprintf("邀请用户赠送 %s", logger.LogQuota(common.QuotaForInviter)))
			_ = inviteUser(inviterId)
		}
	}
	return nil
}

// InsertWithTx inserts a new user within an existing transaction.
// This is used for OAuth registration where user creation and binding need to be atomic.
// Post-creation tasks (sidebar config, logs, inviter rewards) are handled after the transaction commits.
func (user *User) InsertWithTx(tx *gorm.DB, inviterId int) error {
	var err error
	if user.Password != "" {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	user.Quota = common.QuotaForNewUser
	user.AffCode = common.GetRandomString(4)

	// 初始化用户设置
	if user.Setting == "" {
		defaultSetting := dto.UserSetting{}
		user.SetSetting(defaultSetting)
	}

	result := tx.Create(user)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// FinalizeOAuthUserCreation performs post-transaction tasks for OAuth user creation.
// This should be called after the transaction commits successfully.
func (user *User) FinalizeOAuthUserCreation(inviterId int) {
	// 用户创建成功后，根据角色初始化边栏配置
	var createdUser User
	if err := DB.Where("id = ?", user.Id).First(&createdUser).Error; err == nil {
		defaultSidebarConfig := generateDefaultSidebarConfigForRole(createdUser.Role)
		if defaultSidebarConfig != "" {
			currentSetting := createdUser.GetSetting()
			currentSetting.SidebarModules = defaultSidebarConfig
			createdUser.SetSetting(currentSetting)
			createdUser.Update(false)
			common.SysLog(fmt.Sprintf("为新用户 %s (角色: %d) 初始化边栏配置", createdUser.Username, createdUser.Role))
		}
	}

	if common.QuotaForNewUser > 0 {
		RecordLog(user.Id, LogTypeSystem, fmt.Sprintf("新用户注册赠送 %s", logger.LogQuota(common.QuotaForNewUser)))
	}
	if inviterId != 0 {
		if common.QuotaForInvitee > 0 {
			_ = IncreaseUserQuota(user.Id, common.QuotaForInvitee, true)
			RecordLog(user.Id, LogTypeSystem, fmt.Sprintf("使用邀请码赠送 %s", logger.LogQuota(common.QuotaForInvitee)))
		}
		if common.QuotaForInviter > 0 {
			RecordLog(inviterId, LogTypeSystem, fmt.Sprintf("邀请用户赠送 %s", logger.LogQuota(common.QuotaForInviter)))
			_ = inviteUser(inviterId)
		}
	}
}

func (user *User) Update(updatePassword bool) error {
	var err error
	if updatePassword {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	newUser := *user
	DB.First(&user, user.Id)
	if err = DB.Model(user).Updates(newUser).Error; err != nil {
		return err
	}

	// Update cache
	return updateUserCache(*user)
}

func (user *User) Edit(updatePassword bool) error {
	var err error
	if updatePassword {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}

	newUser := *user
	updates := map[string]interface{}{
		"username":     newUser.Username,
		"display_name": newUser.DisplayName,
		"group":        newUser.Group,
		"quota":        newUser.Quota,
		"remark":       newUser.Remark,
		"rebate_rate":  newUser.RebateRate,
	}
	if updatePassword {
		updates["password"] = newUser.Password
	}

	DB.First(&user, user.Id)
	if err = DB.Model(user).Updates(updates).Error; err != nil {
		return err
	}

	// Update cache
	return updateUserCache(*user)
}

func (user *User) ClearBinding(bindingType string) error {
	if user.Id == 0 {
		return errors.New("user id is empty")
	}

	bindingColumnMap := map[string]string{
		"email":    "email",
		"github":   "github_id",
		"discord":  "discord_id",
		"oidc":     "oidc_id",
		"wechat":   "wechat_id",
		"telegram": "telegram_id",
		"linuxdo":  "linux_do_id",
	}

	column, ok := bindingColumnMap[bindingType]
	if !ok {
		return errors.New("invalid binding type")
	}

	if err := DB.Model(&User{}).Where("id = ?", user.Id).Update(column, "").Error; err != nil {
		return err
	}

	if err := DB.Where("id = ?", user.Id).First(user).Error; err != nil {
		return err
	}

	return updateUserCache(*user)
}

func (user *User) Delete() error {
	if user.Id == 0 {
		return errors.New("id 为空！")
	}
	if err := DB.Delete(user).Error; err != nil {
		return err
	}

	// 清除缓存
	return invalidateUserCache(user.Id)
}

func (user *User) HardDelete() error {
	if user.Id == 0 {
		return errors.New("id 为空！")
	}
	err := DB.Unscoped().Delete(user).Error
	return err
}

// ValidateAndFill check password & user status
func (user *User) ValidateAndFill() (err error) {
	// When querying with struct, GORM will only query with non-zero fields,
	// that means if your field's value is 0, '', false or other zero values,
	// it won't be used to build query conditions
	password := user.Password
	username := strings.TrimSpace(user.Username)
	if username == "" || password == "" {
		return errors.New("用户名或密码为空")
	}
	// find buy username or email
	DB.Where("username = ? OR email = ?", username, username).First(user)
	okay := common.ValidatePasswordAndHash(password, user.Password)
	if !okay || user.Status != common.UserStatusEnabled {
		return errors.New("用户名或密码错误，或用户已被封禁")
	}
	return nil
}

func (user *User) FillUserById() error {
	if user.Id == 0 {
		return errors.New("id 为空！")
	}
	DB.Where(User{Id: user.Id}).First(user)
	return nil
}

func (user *User) FillUserByEmail() error {
	if user.Email == "" {
		return errors.New("email 为空！")
	}
	DB.Where(User{Email: user.Email}).First(user)
	return nil
}

func (user *User) FillUserByGitHubId() error {
	if user.GitHubId == "" {
		return errors.New("GitHub id 为空！")
	}
	DB.Where(User{GitHubId: user.GitHubId}).First(user)
	return nil
}

// UpdateGitHubId updates the user's GitHub ID (used for migration from login to numeric ID)
func (user *User) UpdateGitHubId(newGitHubId string) error {
	if user.Id == 0 {
		return errors.New("user id is empty")
	}
	return DB.Model(user).Update("github_id", newGitHubId).Error
}

func (user *User) FillUserByDiscordId() error {
	if user.DiscordId == "" {
		return errors.New("discord id 为空！")
	}
	DB.Where(User{DiscordId: user.DiscordId}).First(user)
	return nil
}

func (user *User) FillUserByOidcId() error {
	if user.OidcId == "" {
		return errors.New("oidc id 为空！")
	}
	DB.Where(User{OidcId: user.OidcId}).First(user)
	return nil
}

func (user *User) FillUserByWeChatId() error {
	if user.WeChatId == "" {
		return errors.New("WeChat id 为空！")
	}
	DB.Where(User{WeChatId: user.WeChatId}).First(user)
	return nil
}

func (user *User) FillUserByTelegramId() error {
	if user.TelegramId == "" {
		return errors.New("Telegram id 为空！")
	}
	err := DB.Where(User{TelegramId: user.TelegramId}).First(user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("该 Telegram 账户未绑定")
	}
	return nil
}

func IsEmailAlreadyTaken(email string) bool {
	return DB.Unscoped().Where("email = ?", email).Find(&User{}).RowsAffected == 1
}

func IsWeChatIdAlreadyTaken(wechatId string) bool {
	return DB.Unscoped().Where("wechat_id = ?", wechatId).Find(&User{}).RowsAffected == 1
}

func IsGitHubIdAlreadyTaken(githubId string) bool {
	return DB.Unscoped().Where("github_id = ?", githubId).Find(&User{}).RowsAffected == 1
}

func IsDiscordIdAlreadyTaken(discordId string) bool {
	return DB.Unscoped().Where("discord_id = ?", discordId).Find(&User{}).RowsAffected == 1
}

func IsOidcIdAlreadyTaken(oidcId string) bool {
	return DB.Where("oidc_id = ?", oidcId).Find(&User{}).RowsAffected == 1
}

func IsTelegramIdAlreadyTaken(telegramId string) bool {
	return DB.Unscoped().Where("telegram_id = ?", telegramId).Find(&User{}).RowsAffected == 1
}

func ResetUserPasswordByEmail(email string, password string) error {
	if email == "" || password == "" {
		return errors.New("邮箱地址或密码为空！")
	}
	hashedPassword, err := common.Password2Hash(password)
	if err != nil {
		return err
	}
	err = DB.Model(&User{}).Where("email = ?", email).Update("password", hashedPassword).Error
	return err
}

func IsAdmin(userId int) bool {
	if userId == 0 {
		return false
	}
	var user User
	err := DB.Where("id = ?", userId).Select("role").Find(&user).Error
	if err != nil {
		common.SysLog("no such user " + err.Error())
		return false
	}
	return user.Role >= common.RoleAdminUser
}

//// IsUserEnabled checks user status from Redis first, falls back to DB if needed
//func IsUserEnabled(id int, fromDB bool) (status bool, err error) {
//	defer func() {
//		// Update Redis cache asynchronously on successful DB read
//		if shouldUpdateRedis(fromDB, err) {
//			gopool.Go(func() {
//				if err := updateUserStatusCache(id, status); err != nil {
//					common.SysError("failed to update user status cache: " + err.Error())
//				}
//			})
//		}
//	}()
//	if !fromDB && common.RedisEnabled {
//		// Try Redis first
//		status, err := getUserStatusCache(id)
//		if err == nil {
//			return status == common.UserStatusEnabled, nil
//		}
//		// Don't return error - fall through to DB
//	}
//	fromDB = true
//	var user User
//	err = DB.Where("id = ?", id).Select("status").Find(&user).Error
//	if err != nil {
//		return false, err
//	}
//
//	return user.Status == common.UserStatusEnabled, nil
//}

func ValidateAccessToken(token string) (user *User) {
	if token == "" {
		return nil
	}
	token = strings.Replace(token, "Bearer ", "", 1)
	user = &User{}
	if DB.Where("access_token = ?", token).First(user).RowsAffected == 1 {
		return user
	}
	return nil
}

// GetUserQuota gets quota from Redis first, falls back to DB if needed
func GetUserQuota(id int, fromDB bool) (quota int, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) {
			gopool.Go(func() {
				if err := updateUserQuotaCache(id, quota); err != nil {
					common.SysLog("failed to update user quota cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		quota, err := getUserQuotaCache(id)
		if err == nil {
			return quota, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = DB.Model(&User{}).Where("id = ?", id).Select("quota").Find(&quota).Error
	if err != nil {
		return 0, err
	}

	return quota, nil
}

func GetUserUsedQuota(id int) (quota int, err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Select("used_quota").Find(&quota).Error
	return quota, err
}

func GetUserEmail(id int) (email string, err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Select("email").Find(&email).Error
	return email, err
}

// GetUserGroup gets group from Redis first, falls back to DB if needed
func GetUserGroup(id int, fromDB bool) (group string, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) {
			gopool.Go(func() {
				if err := updateUserGroupCache(id, group); err != nil {
					common.SysLog("failed to update user group cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		group, err := getUserGroupCache(id)
		if err == nil {
			return group, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = DB.Model(&User{}).Where("id = ?", id).Select(commonGroupCol).Find(&group).Error
	if err != nil {
		return "", err
	}

	return group, nil
}

// GetUserSetting gets setting from Redis first, falls back to DB if needed
func GetUserSetting(id int, fromDB bool) (settingMap dto.UserSetting, err error) {
	var setting string
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) {
			gopool.Go(func() {
				if err := updateUserSettingCache(id, setting); err != nil {
					common.SysLog("failed to update user setting cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		setting, err := getUserSettingCache(id)
		if err == nil {
			return setting, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	// can be nil setting
	var safeSetting sql.NullString
	err = DB.Model(&User{}).Where("id = ?", id).Select("setting").Find(&safeSetting).Error
	if err != nil {
		return settingMap, err
	}
	if safeSetting.Valid {
		setting = safeSetting.String
	} else {
		setting = ""
	}
	userBase := &UserBase{
		Setting: setting,
	}
	return userBase.GetSetting(), nil
}

func IncreaseUserQuota(id int, quota int, db bool) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	gopool.Go(func() {
		err := cacheIncrUserQuota(id, int64(quota))
		if err != nil {
			common.SysLog("failed to increase user quota: " + err.Error())
		}
	})
	if !db && common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUserQuota, id, quota)
		return nil
	}
	return increaseUserQuota(id, quota)
}

func increaseUserQuota(id int, quota int) (err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Update("quota", gorm.Expr("quota + ?", quota)).Error
	if err != nil {
		return err
	}
	return err
}

func DecreaseUserQuota(id int, quota int) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	gopool.Go(func() {
		err := cacheDecrUserQuota(id, int64(quota))
		if err != nil {
			common.SysLog("failed to decrease user quota: " + err.Error())
		}
	})
	if common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUserQuota, id, -quota)
		return nil
	}
	return decreaseUserQuota(id, quota)
}

func decreaseUserQuota(id int, quota int) (err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Update("quota", gorm.Expr("quota - ?", quota)).Error
	if err != nil {
		return err
	}
	return err
}

func DeltaUpdateUserQuota(id int, delta int) (err error) {
	if delta == 0 {
		return nil
	}
	if delta > 0 {
		return IncreaseUserQuota(id, delta, false)
	} else {
		return DecreaseUserQuota(id, -delta)
	}
}

//func GetRootUserEmail() (email string) {
//	DB.Model(&User{}).Where("role = ?", common.RoleRootUser).Select("email").Find(&email)
//	return email
//}

func GetRootUser() (user *User) {
	DB.Where("role = ?", common.RoleRootUser).First(&user)
	return user
}

func UpdateUserUsedQuotaAndRequestCount(id int, quota int) {
	if common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUsedQuota, id, quota)
		addNewRecord(BatchUpdateTypeRequestCount, id, 1)
		return
	}
	updateUserUsedQuotaAndRequestCount(id, quota, 1)
}

func updateUserUsedQuotaAndRequestCount(id int, quota int, count int) {
	err := DB.Model(&User{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"request_count": gorm.Expr("request_count + ?", count),
		},
	).Error
	if err != nil {
		common.SysLog("failed to update user used quota and request count: " + err.Error())
		return
	}

	//// 更新缓存
	//if err := invalidateUserCache(id); err != nil {
	//	common.SysError("failed to invalidate user cache: " + err.Error())
	//}
}

func updateUserUsedQuota(id int, quota int) {
	err := DB.Model(&User{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota": gorm.Expr("used_quota + ?", quota),
		},
	).Error
	if err != nil {
		common.SysLog("failed to update user used quota: " + err.Error())
	}
}

func updateUserRequestCount(id int, count int) {
	err := DB.Model(&User{}).Where("id = ?", id).Update("request_count", gorm.Expr("request_count + ?", count)).Error
	if err != nil {
		common.SysLog("failed to update user request count: " + err.Error())
	}
}

// GetUsernameById gets username from Redis first, falls back to DB if needed
func GetUsernameById(id int, fromDB bool) (username string, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) {
			gopool.Go(func() {
				if err := updateUserNameCache(id, username); err != nil {
					common.SysLog("failed to update user name cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		username, err := getUserNameCache(id)
		if err == nil {
			return username, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = DB.Model(&User{}).Where("id = ?", id).Select("username").Find(&username).Error
	if err != nil {
		return "", err
	}

	return username, nil
}

func IsLinuxDOIdAlreadyTaken(linuxDOId string) bool {
	var user User
	err := DB.Unscoped().Where("linux_do_id = ?", linuxDOId).First(&user).Error
	return !errors.Is(err, gorm.ErrRecordNotFound)
}

func (user *User) FillUserByLinuxDOId() error {
	if user.LinuxDOId == "" {
		return errors.New("linux do id is empty")
	}
	err := DB.Where("linux_do_id = ?", user.LinuxDOId).First(user).Error
	return err
}

func RootUserExists() bool {
	var user User
	err := DB.Where("role = ?", common.RoleRootUser).First(&user).Error
	if err != nil {
		return false
	}
	return true
}

// AgentSubUserItem is a minimized, safe projection for agent sub-user listing.
// It deliberately excludes sensitive fields such as access_token/email/oauth ids.
type AgentSubUserItem struct {
	Id              int            `json:"id" gorm:"column:id"`
	Username        string         `json:"-" gorm:"column:username"`
	DisplayName     string         `json:"display_name" gorm:"column:display_name"`
	Role            int            `json:"role" gorm:"column:role"`
	Group           string         `json:"group" gorm:"column:group"`
	InviterId       int            `json:"inviter_id" gorm:"column:inviter_id"`
	Quota           int            `json:"quota" gorm:"column:quota"`
	UsedQuota       int            `json:"used_quota" gorm:"column:used_quota"`
	RequestCount    int            `json:"request_count" gorm:"column:request_count"`
	AffCount        int            `json:"aff_count" gorm:"column:aff_count"`
	AffHistoryQuota int            `json:"aff_history_quota" gorm:"column:aff_history"`
	RegisteredAt    int64          `json:"registered_at" gorm:"column:created_at"`
	Status          int            `json:"status" gorm:"column:status"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"column:deleted_at"`
}

// GetAgentSubUsers returns paginated list of users invited by the agent.
// NOTE: returns a safe DTO instead of full User entity to avoid data leakage.
func GetAgentSubUsers(agentId int, pageInfo *common.PageInfo) (users []AgentSubUserItem, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	baseQuery := tx.Unscoped().Table("users").Where("inviter_id = ?", agentId)

	err = baseQuery.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	err = baseQuery.
		Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&users).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// SearchAgentSubUsers searches sub-users by keyword, scoped to an agent's invitees.
// NOTE: returns a safe DTO instead of full User entity to avoid data leakage.
func SearchAgentSubUsers(agentId int, keyword string, startIdx int, num int) ([]AgentSubUserItem, int64, error) {
	var users []AgentSubUserItem
	var total int64

	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Unscoped().Table("users").Where("inviter_id = ?", agentId)

	if keyword != "" {
		likeCondition := "username LIKE ? OR display_name LIKE ?"
		keywordInt, convErr := strconv.Atoi(keyword)
		if convErr == nil {
			likeCondition = "id = ? OR " + likeCondition
			query = query.Where(likeCondition,
				keywordInt, "%"+keyword+"%", "%"+keyword+"%")
		} else {
			query = query.Where(likeCondition,
				"%"+keyword+"%", "%"+keyword+"%")
		}
	}

	err := query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&users).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// GetAgentSubUserIDs returns all user IDs invited by the given agent
func GetAgentSubUserIDs(agentId int) ([]int, error) {
	var ids []int
	err := DB.Model(&User{}).Where("inviter_id = ?", agentId).Pluck("id", &ids).Error
	return ids, err
}

// AgentDashboardStats holds the dashboard statistics for an agent
type AgentDashboardStats struct {
	TodayTopup              float64 `json:"today_topup"`
	TodayConsumption        int     `json:"today_consumption"`
	TodayRegistrations      int64   `json:"today_registrations"`
	TodayAgentRegistrations int64   `json:"today_agent_registrations"`
	RangeRebateMoney        float64 `json:"range_rebate_money"`
	RangeRebateQuota        int64   `json:"range_rebate_quota"`
	TotalTopup              float64 `json:"total_topup"`
	TotalConsumption        int     `json:"total_consumption"`
	TotalRegistrations      int64   `json:"total_registrations"`
	TotalAgentRegistrations int64   `json:"total_agent_registrations"`
	TotalRebateMoney        float64 `json:"total_rebate_money"`
	TotalRebateQuota        int64   `json:"total_rebate_quota"`
	TotalSubUsers           int64   `json:"total_sub_users"`
	// rankings
	ModelRanking   []AgentRankItem `json:"model_ranking"`
	UserRanking    []AgentRankItem `json:"user_ranking"`
	ChannelRanking []AgentRankItem `json:"channel_ranking"`
	ErrorRanking   []AgentRankItem `json:"error_ranking"`
}

type AgentRankItem struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func userHasCreatedAtColumn() bool {
	if DB == nil {
		return false
	}
	return DB.Migrator().HasColumn(&User{}, "created_at")
}

// GetAgentDashboardStats computes dashboard stats scoped to the agent's sub-users
func GetAgentDashboardStats(agentId int, startTimestamp, endTimestamp int64) (*AgentDashboardStats, error) {
	stats := &AgentDashboardStats{}

	subUsersSubQuery := DB.Model(&User{}).Select("id").Where("inviter_id = ?", agentId)
	var subUserIDs []int

	// Total sub-users count
	DB.Model(&User{}).Where("inviter_id = ?", agentId).Count(&stats.TotalSubUsers)
	DB.Model(&User{}).Where("inviter_id = ? AND role = ?", agentId, common.RoleAgentUser).Count(&stats.TotalAgentRegistrations)
	stats.TotalRegistrations = stats.TotalSubUsers

	if startTimestamp == 0 {
		now := time.Now()
		startTimestamp = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	}
	if endTimestamp == 0 {
		endTimestamp = time.Now().Unix()
	}

	// created_at is not guaranteed on all historical databases.
	// Guard these queries to avoid runtime SQL errors on instances without this column.
	if userHasCreatedAtColumn() {
		DB.Model(&User{}).Where("inviter_id = ? AND created_at >= ? AND created_at <= ?",
			agentId, startTimestamp, endTimestamp).Count(&stats.TodayRegistrations)

		DB.Model(&User{}).Where("inviter_id = ? AND role = ? AND created_at >= ? AND created_at <= ?",
			agentId, common.RoleAgentUser, startTimestamp, endTimestamp).Count(&stats.TodayAgentRegistrations)
	}

	if stats.TotalSubUsers == 0 {
		stats.ModelRanking = []AgentRankItem{}
		stats.UserRanking = []AgentRankItem{}
		stats.ChannelRanking = []AgentRankItem{}
		stats.ErrorRanking = []AgentRankItem{}
		return stats, nil
	}

	// Today top-up money from successful top-up orders of sub-users.
	var todayOnlineTopup float64
	DB.Model(&TopUp{}).Select("COALESCE(sum(money), 0)").
		Where("user_id IN (?) AND status = ? AND complete_time >= ? AND complete_time <= ?",
			subUsersSubQuery, common.TopUpStatusSuccess, startTimestamp, endTimestamp).
		Scan(&todayOnlineTopup)
	var todayRedeemQuota int64
	DB.Model(&Redemption{}).Select("COALESCE(sum(quota), 0)").
		Where("used_user_id IN (?) AND status = ? AND redeemed_time >= ? AND redeemed_time <= ?",
			subUsersSubQuery, common.RedemptionCodeStatusUsed, startTimestamp, endTimestamp).
		Scan(&todayRedeemQuota)
	stats.TodayTopup = todayOnlineTopup + float64(todayRedeemQuota)/common.QuotaPerUnit

	// Total top-up money from successful top-up orders of sub-users.
	var totalOnlineTopup float64
	DB.Model(&TopUp{}).Select("COALESCE(sum(money), 0)").
		Where("user_id IN (?) AND status = ?", subUsersSubQuery, common.TopUpStatusSuccess).
		Scan(&totalOnlineTopup)
	var totalRedeemQuota int64
	DB.Model(&Redemption{}).Select("COALESCE(sum(quota), 0)").
		Where("used_user_id IN (?) AND status = ?", subUsersSubQuery, common.RedemptionCodeStatusUsed).
		Scan(&totalRedeemQuota)
	stats.TotalTopup = totalOnlineTopup + float64(totalRedeemQuota)/common.QuotaPerUnit

	// Agent rebate totals for the selected range and overall.
	type rebateAggregateRow struct {
		RebateMoney float64 `gorm:"column:rebate_money"`
		RebateQuota int64   `gorm:"column:rebate_quota"`
	}
	var rangeRebateRow rebateAggregateRow
	DB.Model(&AgentRebateRecord{}).
		Where("agent_id = ? AND created_at >= ? AND created_at <= ?", agentId, startTimestamp, endTimestamp).
		Select("COALESCE(sum(rebate_money), 0) AS rebate_money, COALESCE(sum(rebate_quota), 0) AS rebate_quota").
		Scan(&rangeRebateRow)
	stats.RangeRebateMoney = rangeRebateRow.RebateMoney
	stats.RangeRebateQuota = rangeRebateRow.RebateQuota

	var totalRebateRow rebateAggregateRow
	DB.Model(&AgentRebateRecord{}).
		Where("agent_id = ?", agentId).
		Select("COALESCE(sum(rebate_money), 0) AS rebate_money, COALESCE(sum(rebate_quota), 0) AS rebate_quota").
		Scan(&totalRebateRow)
	stats.TotalRebateMoney = totalRebateRow.RebateMoney
	stats.TotalRebateQuota = totalRebateRow.RebateQuota

	// LOG_DB may be a standalone database (LOG_SQL_DSN).
	// In that case, cross-database subquery against users table is unavailable.
	useSubQueryOnLogDB := LOG_DB == DB
	if !useSubQueryOnLogDB {
		ids, err := GetAgentSubUserIDs(agentId)
		if err != nil {
			return nil, err
		}
		subUserIDs = ids
	}

	// Today consumption (sum of quota from logs)
	consumeQuery := LOG_DB.Table("logs").Select("COALESCE(sum(quota), 0)").
		Where("type = ? AND created_at >= ? AND created_at <= ?",
			LogTypeConsume, startTimestamp, endTimestamp)
	if useSubQueryOnLogDB {
		consumeQuery = consumeQuery.Where("user_id IN (?)", subUsersSubQuery)
	} else {
		consumeQuery = consumeQuery.Where("user_id IN ?", subUserIDs)
	}
	consumeQuery.
		Scan(&stats.TodayConsumption)

	// Total consumption (sum of quota from logs)
	totalConsumeQuery := LOG_DB.Table("logs").Select("COALESCE(sum(quota), 0)").
		Where("type = ?", LogTypeConsume)
	if useSubQueryOnLogDB {
		totalConsumeQuery = totalConsumeQuery.Where("user_id IN (?)", subUsersSubQuery)
	} else {
		totalConsumeQuery = totalConsumeQuery.Where("user_id IN ?", subUserIDs)
	}
	totalConsumeQuery.
		Scan(&stats.TotalConsumption)

	// Model ranking
	var modelRanks []AgentRankItem
	modelRankQuery := LOG_DB.Table("logs").Select("model_name as name, COALESCE(sum(quota), 0) as value").
		Where("type = ? AND created_at >= ? AND created_at <= ?",
			LogTypeConsume, startTimestamp, endTimestamp)
	if useSubQueryOnLogDB {
		modelRankQuery = modelRankQuery.Where("user_id IN (?)", subUsersSubQuery)
	} else {
		modelRankQuery = modelRankQuery.Where("user_id IN ?", subUserIDs)
	}
	modelRankQuery.
		Group("model_name").Order("value desc").Limit(10).Scan(&modelRanks)
	if modelRanks == nil {
		modelRanks = []AgentRankItem{}
	}
	stats.ModelRanking = modelRanks

	// User ranking
	var userRanks []AgentRankItem
	userRankQuery := LOG_DB.Table("logs").Select("username as name, COALESCE(sum(quota), 0) as value").
		Where("type = ? AND created_at >= ? AND created_at <= ?",
			LogTypeConsume, startTimestamp, endTimestamp)
	if useSubQueryOnLogDB {
		userRankQuery = userRankQuery.Where("user_id IN (?)", subUsersSubQuery)
	} else {
		userRankQuery = userRankQuery.Where("user_id IN ?", subUserIDs)
	}
	userRankQuery.
		Group("username").Order("value desc").Limit(10).Scan(&userRanks)
	if userRanks == nil {
		userRanks = []AgentRankItem{}
	}
	stats.UserRanking = userRanks

	// Channel ranking
	type channelRankRow struct {
		ChannelId int `json:"channel_id"`
		Value     int `json:"value"`
	}
	var channelRows []channelRankRow
	channelRankQuery := LOG_DB.Table("logs").Select("channel_id, COALESCE(sum(quota), 0) as value").
		Where("type = ? AND created_at >= ? AND created_at <= ?",
			LogTypeConsume, startTimestamp, endTimestamp)
	if useSubQueryOnLogDB {
		channelRankQuery = channelRankQuery.Where("user_id IN (?)", subUsersSubQuery)
	} else {
		channelRankQuery = channelRankQuery.Where("user_id IN ?", subUserIDs)
	}
	channelRankQuery.
		Group("channel_id").Order("value desc").Limit(10).Scan(&channelRows)
	channelRanks := make([]AgentRankItem, 0, len(channelRows))
	for _, row := range channelRows {
		channelRanks = append(channelRanks, AgentRankItem{
			Name:  fmt.Sprintf("%d", row.ChannelId),
			Value: row.Value,
		})
	}
	stats.ChannelRanking = channelRanks

	// Error model ranking
	var errorRanks []AgentRankItem
	errorRankQuery := LOG_DB.Table("logs").Select("model_name as name, count(*) as value").
		Where("type = ? AND created_at >= ? AND created_at <= ?",
			LogTypeError, startTimestamp, endTimestamp)
	if useSubQueryOnLogDB {
		errorRankQuery = errorRankQuery.Where("user_id IN (?)", subUsersSubQuery)
	} else {
		errorRankQuery = errorRankQuery.Where("user_id IN ?", subUserIDs)
	}
	errorRankQuery.
		Group("model_name").Order("value desc").Limit(10).Scan(&errorRanks)
	if errorRanks == nil {
		errorRanks = []AgentRankItem{}
	}
	stats.ErrorRanking = errorRanks

	return stats, nil
}
