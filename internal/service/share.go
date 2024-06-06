package service

import (
	v1 "PandoraHelper/api/v1"
	"PandoraHelper/internal/model"
	"PandoraHelper/internal/repository"
	"context"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"strconv"
	"strings"
	"time"
)

type ShareService interface {
	RefreshShareToken(ctx context.Context, share *model.Share, accessToken string, resetLimit bool) (string, error)
	ResetShareLimit(ctx context.Context, id int64) error
	GetShare(ctx context.Context, id int64) (*model.Share, error)
	Update(ctx context.Context, share *model.Share) error
	Create(ctx context.Context, share *model.Share) error
	SearchShare(ctx context.Context, email string, uniqueName string) ([]*model.Share, error)
	DeleteShare(ctx context.Context, id int64) error
	LoginShareByPassword(ctx context.Context, username string, password string) (string, error)
	ShareStatistic(ctx context.Context, accountId int) (interface{}, interface{})
	ShareResetPassword(ctx context.Context, uniqueName string, password string, newPassword string, confirmNewPassword string) error
	GetSharesByAccountId(ctx context.Context, accountId int) ([]*model.Share, error)
}

func NewShareService(service *Service, shareRepository repository.ShareRepository, viper *viper.Viper, coordinator *Coordinator) ShareService {
	return &shareService{
		Service:         service,
		shareRepository: shareRepository,
		viper:           viper,
		accountService:  coordinator.AccountSvc,
	}
}

type shareService struct {
	*Service
	shareRepository repository.ShareRepository
	viper           *viper.Viper
	accountService  AccountService
}

func (s *shareService) GetSharesByAccountId(ctx context.Context, accountId int) ([]*model.Share, error) {
	return s.shareRepository.GetSharesByAccountId(ctx, accountId)
}

func (s *shareService) ShareResetPassword(ctx context.Context, uniqueName string, password string, newPassword string, confirmNewPassword string) error {
	share, err := s.shareRepository.GetShareByUniqueName(ctx, uniqueName)
	if err != nil {
		return err
	}
	if share.Password != password {
		return v1.ErrUsernameOrPassword
	}
	if newPassword != confirmNewPassword {
		return v1.ErrPasswordNotMatch
	}
	share.Password = newPassword
	err = s.shareRepository.Update(ctx, share)
	if err != nil {
		return err
	}
	return nil
}

// ShareStatistic 转换为Go语言
func (s *shareService) ShareStatistic(ctx context.Context, accountId int) (interface{}, interface{}) {
	account, err := s.accountService.GetAccount(ctx, int64(accountId))
	if err != nil {
		return nil, err
	}
	shares := account.Shares

	uniqueNames := make([]string, 0)
	gpt35Counts := make([]int, 0)
	gpt4Counts := make([]int, 0)

	for _, share := range shares {
		uniqueNames = append(uniqueNames, share.UniqueName)
		gpt35count, gpt4Count, err := s.GetShareTokenInfo(share.ShareToken, account.AccessToken)
		if err != nil {
			return nil, nil
		}
		gpt35Counts = append(gpt35Counts, gpt35count)
		gpt4Counts = append(gpt4Counts, gpt4Count)
	}

	series := []map[string]interface{}{
		{
			"name": "GPT-3.5",
			"data": gpt35Counts,
		},
		{
			"name": "GPT-4",
			"data": gpt4Counts,
		},
	}

	return map[string]interface{}{
		"categories": uniqueNames,
		"series":     series,
	}, nil

}

func (s *shareService) LoginShareByPassword(ctx context.Context, username string, password string) (string, error) {
	share, err := s.shareRepository.GetShareByUniqueName(ctx, username)
	if err != nil {
		return "", v1.ErrUsernameOrPassword
	}
	if share.Password != password {
		return "", v1.ErrUsernameOrPassword
	}

	indexDomain := fmt.Sprintf("%s/api/auth/oauth_token", s.viper.GetString("pandora.domain.index"))
	reqBody := map[string]interface{}{
		"share_token": share.ShareToken,
	}
	result := struct {
		LoginUrl   string `json:"login_url"`
		OauthToken string `json:"oauth_token"`
	}{}
	client := resty.New()
	_, err = client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(reqBody).
		SetResult(&result).
		Post(indexDomain)
	if err != nil {
		s.logger.Error("LoginShareByPassword error", zap.Any("err", err))
		return "", err
	}
	s.logger.Info("LoginShareByPassword resp", zap.Any("resp", result))
	finalLoginUrl := fmt.Sprintf("%s/auth/login_oauth?token=%s", s.viper.GetString("pandora.domain.index"), result.OauthToken)
	return finalLoginUrl, nil
}

func (s *shareService) GetShareTokenByAccessToken(accessToken string, share *model.Share, resetLimit bool) (string, error) {
	chatDomain := fmt.Sprintf("%s/token/register", s.viper.GetString("pandora.domain.chat"))
	var resp struct {
		TokenKey string `json:"token_key"`
	}
	client := resty.New()
	_, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormData(map[string]string{
			"unique_name":        share.UniqueName,
			"access_token":       accessToken,
			"expires_in":         fmt.Sprintf("%d", share.ExpiresIn),
			"site_limit":         share.SiteLimit,
			"reset_limit":        fmt.Sprintf("%t", resetLimit),
			"show_conversations": fmt.Sprintf("%t", !share.ShowConversations),
			"show_userinfo":      fmt.Sprintf("%t", share.ShowUserinfo),
			"temporary_chat":     fmt.Sprintf("%t", share.TemporaryChat),
			"gpt35_limit":        fmt.Sprintf("%d", share.Gpt35Limit),
			"gpt4_limit":         fmt.Sprintf("%d", share.Gpt4Limit),
		}).
		SetResult(&resp).
		Post(chatDomain)
	if err != nil {
		s.logger.Error("RefreshShareToken error", zap.Any("err", err))
		return "", err
	}
	s.logger.Info("RefreshShareToken resp", zap.Any("resp", resp))
	return resp.TokenKey, nil
}

func (s *shareService) RefreshShareToken(ctx context.Context, share *model.Share, accessToken string, resetLimit bool) (string, error) {
	if accessToken == "" {
		account, err := s.accountService.GetAccount(ctx, int64(share.AccountID))
		if err != nil {
			return "", err
		}
		accessToken = account.AccessToken
	}
	// 判断ExpiresAt YYYY-MM-DD 23:59:59
	if share.ExpiresAt != "" {
		atExp, err := s.jwt.ParseTokenExp(accessToken)
		if err != nil {
			return "", err
		}
		now := time.Now().Unix()
		shareExp, err := time.Parse("2006-01-02 15:04:05", share.ExpiresAt+" 23:59:59")
		if err != nil {
			return "", err
		}
		shareExpUnix := shareExp.Unix()
		// 如果 过期日期 大于 AccessToken的过期日期，则将ExpiresIn设置0
		if shareExpUnix > atExp {
			share.ExpiresIn = 0
		} else if shareExpUnix > now {
			// 过期时间大于当前时间，小于AccessToken的过期时间，设置ExpiresIn
			share.ExpiresIn = int(shareExpUnix - now)
		} else {
			// 过期时间小于当前时间，已过期
			// 如果备注为[已过期]开头，则不再添加
			if strings.HasPrefix(share.Comment, "[已过期]") {
				return "", nil
			}
			share.ExpiresIn = -1
			share.Comment = "[已过期]" + share.Comment
			err := s.shareRepository.Update(ctx, share)
			if err != nil {
				return "", err
			}
			return "", nil
		}
	}

	return s.GetShareTokenByAccessToken(accessToken, share, resetLimit)
}

func (s *shareService) Update(ctx context.Context, share *model.Share) error {
	_, err := s.RefreshShareToken(ctx, share, "", false)
	if err != nil {
		return err
	}
	err = s.shareRepository.Update(ctx, share)
	if err != nil {
		return err
	}
	return nil
}

func (s *shareService) Create(ctx context.Context, share *model.Share) error {
	token, err := s.RefreshShareToken(ctx, share, "", false)
	if err != nil {
		return err
	}
	share.ShareToken = token
	err = s.shareRepository.Create(ctx, share)
	return nil
}

func (s *shareService) SearchShare(ctx context.Context, email string, uniqueName string) ([]*model.Share, error) {
	return s.shareRepository.SearchShare(ctx, email, uniqueName)
}

func (s *shareService) DeleteShare(ctx context.Context, id int64) error {
	share, err := s.GetShare(ctx, id)
	if err != nil {
		return err
	}
	share.ExpiresIn = -1
	_, err = s.RefreshShareToken(ctx, share, "", false)
	if err != nil {
		return err
	}
	return s.shareRepository.DeleteShare(ctx, id)
}

func (s *shareService) GetShare(ctx context.Context, id int64) (*model.Share, error) {
	return s.shareRepository.GetShare(ctx, id)
}

func (s *shareService) ResetShareLimit(ctx context.Context, id int64) error {
	share, err := s.GetShare(ctx, id)
	if err != nil {
		return err
	}
	_, err = s.RefreshShareToken(ctx, share, "", true)
	if err != nil {
		return err
	}
	return nil
}

func (s *shareService) GetShareTokenInfo(shareToken string, accessToken string) (int, int, error) {
	host := fmt.Sprintf("%s/token/info/%s", s.viper.GetString("pandora.domain.chat"), shareToken)
	headers := map[string]string{}
	if accessToken != "" {
		headers["Authorization"] = fmt.Sprintf("Bearer %s", accessToken)
	}
	var result struct {
		Gpt35Limit string `json:"gpt35_limit"`
		Gpt4Limit  string `json:"gpt4_limit"`
	}
	client := resty.New()
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeaders(headers).
		SetResult(&result).
		Get(host)

	if err != nil {
		return 0, 0, err
	}

	// 将字符串转换为整数
	gpt35Limit, err := strconv.Atoi(result.Gpt35Limit)
	if err != nil {
		gpt35Limit = 0
	}

	gpt4Limit, err := strconv.Atoi(result.Gpt4Limit)
	if err != nil {
		gpt4Limit = 0
	}
	s.logger.Info("GetShareTokenInfo resp", zap.Any("resp", resp))
	return gpt35Limit, gpt4Limit, nil
}
