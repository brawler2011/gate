package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type EmailService interface {
	SendVerificationEmail(ctx context.Context, toEmail, username, token string) error
	SendPasswordResetEmail(ctx context.Context, toEmail, username, token string) error
	SendEmailChangeVerification(ctx context.Context, toNewEmail, username, token string) error
	SendEmailChangeAlert(ctx context.Context, toOldEmail, username, newEmail string) error
	SendOrgInvitationEmail(ctx context.Context, toEmail, username, orgName, orgLogin, role, inviterUsername string) error
	SendOrgJoinRequestEmail(ctx context.Context, toEmail, adminUsername, applicantUsername, orgName, orgLogin string) error
	SendOrgJoinRequestResolvedEmail(ctx context.Context, toEmail, username, orgName, orgLogin string, approved bool) error
	SendContestJoinRequestEmail(ctx context.Context, toEmail, moderatorUsername, applicantUsername, contestTitle, orgLogin, contestLogin string) error
	SendContestJoinRequestResolvedEmail(ctx context.Context, toEmail, username, contestTitle, orgLogin, contestLogin string, approved bool) error
}

func NewEmailService(envName, apiKey, fromEmail, appBaseURL string) EmailService {
	appBaseURL = strings.TrimRight(appBaseURL, "/")
	if appBaseURL == "" {
		appBaseURL = "http://localhost:3000"
	}
	if fromEmail == "" {
		fromEmail = "Gate <no-reply@gate.local>"
	}

	if envName == "local" {
		slog.Info("running in local environment; using LogEmailService for email delivery", "env", envName)
		return &LogEmailService{
			AppBaseURL: appBaseURL,
		}
	}

	if apiKey == "" {
		slog.Warn("no RESEND_API_KEY provided in non-local environment; falling back to LogEmailService", "env", envName)
		return &LogEmailService{
			AppBaseURL: appBaseURL,
		}
	}

	slog.Info("initializing Resend email delivery service", "env", envName, "from", fromEmail)
	return &ResendEmailService{
		APIKey:     apiKey,
		FromEmail:  fromEmail,
		AppBaseURL: appBaseURL,
		Client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// LogEmailService logs emails via slog for local development and testing
type LogEmailService struct {
	AppBaseURL string
}

func (s *LogEmailService) SendVerificationEmail(ctx context.Context, toEmail, username, token string) error {
	link := fmt.Sprintf("%s/auth/verify-email?token=%s", s.AppBaseURL, token)
	slog.Info("[EMAIL MOCK] Verification email",
		"to", toEmail,
		"username", username,
		"link", link,
	)
	return nil
}

func (s *LogEmailService) SendPasswordResetEmail(ctx context.Context, toEmail, username, token string) error {
	link := fmt.Sprintf("%s/auth/reset-password?token=%s", s.AppBaseURL, token)
	slog.Info("[EMAIL MOCK] Password reset email",
		"to", toEmail,
		"username", username,
		"link", link,
	)
	return nil
}

func (s *LogEmailService) SendEmailChangeVerification(ctx context.Context, toNewEmail, username, token string) error {
	link := fmt.Sprintf("%s/auth/confirm-email-change?token=%s", s.AppBaseURL, token)
	slog.Info("[EMAIL MOCK] Email change verification",
		"to", toNewEmail,
		"username", username,
		"link", link,
	)
	return nil
}

func (s *LogEmailService) SendEmailChangeAlert(ctx context.Context, toOldEmail, username, newEmail string) error {
	slog.Info("[EMAIL MOCK] Email change security alert",
		"to", toOldEmail,
		"username", username,
		"new_email", newEmail,
	)
	return nil
}

func (s *LogEmailService) SendOrgInvitationEmail(ctx context.Context, toEmail, username, orgName, orgLogin, role, inviterUsername string) error {
	link := fmt.Sprintf("%s/notifications", s.AppBaseURL)
	slog.Info("[EMAIL MOCK] Org invitation email",
		"to", toEmail,
		"username", username,
		"org_name", orgName,
		"org_login", orgLogin,
		"role", role,
		"inviter", inviterUsername,
		"link", link,
	)
	return nil
}

func (s *LogEmailService) SendOrgJoinRequestEmail(ctx context.Context, toEmail, adminUsername, applicantUsername, orgName, orgLogin string) error {
	link := fmt.Sprintf("%s/%s/settings/members", s.AppBaseURL, orgLogin)
	slog.Info("[EMAIL MOCK] Org join request email",
		"to", toEmail,
		"admin", adminUsername,
		"applicant", applicantUsername,
		"org_name", orgName,
		"org_login", orgLogin,
		"link", link,
	)
	return nil
}

func (s *LogEmailService) SendOrgJoinRequestResolvedEmail(ctx context.Context, toEmail, username, orgName, orgLogin string, approved bool) error {
	link := fmt.Sprintf("%s/%s", s.AppBaseURL, orgLogin)
	slog.Info("[EMAIL MOCK] Org join request resolved email",
		"to", toEmail,
		"username", username,
		"org_name", orgName,
		"org_login", orgLogin,
		"approved", approved,
		"link", link,
	)
	return nil
}

func (s *LogEmailService) SendContestJoinRequestEmail(ctx context.Context, toEmail, moderatorUsername, applicantUsername, contestTitle, orgLogin, contestLogin string) error {
	link := fmt.Sprintf("%s/%s/%s", s.AppBaseURL, orgLogin, contestLogin)
	slog.Info("[EMAIL MOCK] Contest join request email",
		"to", toEmail,
		"moderator", moderatorUsername,
		"applicant", applicantUsername,
		"contest", contestTitle,
		"org_login", orgLogin,
		"contest_login", contestLogin,
		"link", link,
	)
	return nil
}

func (s *LogEmailService) SendContestJoinRequestResolvedEmail(ctx context.Context, toEmail, username, contestTitle, orgLogin, contestLogin string, approved bool) error {
	link := fmt.Sprintf("%s/%s/%s", s.AppBaseURL, orgLogin, contestLogin)
	slog.Info("[EMAIL MOCK] Contest join request resolved email",
		"to", toEmail,
		"username", username,
		"contest", contestTitle,
		"org_login", orgLogin,
		"contest_login", contestLogin,
		"approved", approved,
		"link", link,
	)
	return nil
}

// ResendEmailService delivers emails using the Resend HTTP API
type ResendEmailService struct {
	APIKey     string
	FromEmail  string
	AppBaseURL string
	Client     *http.Client
}

type resendSendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Html    string   `json:"html"`
	Text    string   `json:"text"`
}

func (s *ResendEmailService) send(ctx context.Context, toEmail, subject, htmlBody, textBody string) error {
	reqBody := resendSendRequest{
		From:    s.FromEmail,
		To:      []string{toEmail},
		Subject: subject,
		Html:    htmlBody,
		Text:    textBody,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal email request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create email request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email via Resend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API returned error status %d: %s", resp.StatusCode, string(respBytes))
	}

	return nil
}

func (s *ResendEmailService) SendVerificationEmail(ctx context.Context, toEmail, username, token string) error {
	link := fmt.Sprintf("%s/auth/verify-email?token=%s", s.AppBaseURL, token)
	subject := "Подтверждение регистрации на платформе Gate"
	htmlBody := fmt.Sprintf(`
		<h2>Здравствуйте, %s!</h2>
		<p>Благодарим за регистрацию на платформе Gate.</p>
		<p>Для подтверждения вашего адреса электронной почты и активации учетной записи, пожалуйста, перейдите по ссылке ниже:</p>
		<p><a href="%s" style="display:inline-block;padding:10px 20px;background-color:#228be6;color:#ffffff;text-decoration:none;border-radius:4px;">Подтвердить почту</a></p>
		<p>Или скопируйте ссылку в адресную строку браузера: <br/><code>%s</code></p>
		<p>Ссылка действительна в течение 24 часов.</p>
	`, username, link, link)
	textBody := fmt.Sprintf("Здравствуйте, %s!\n\nДля подтверждения почты перейдите по ссылке:\n%s\n\nСсылка действительна 24 часа.", username, link)

	return s.send(ctx, toEmail, subject, htmlBody, textBody)
}

func (s *ResendEmailService) SendPasswordResetEmail(ctx context.Context, toEmail, username, token string) error {
	link := fmt.Sprintf("%s/auth/reset-password?token=%s", s.AppBaseURL, token)
	subject := "Сброс пароля на платформе Gate"
	htmlBody := fmt.Sprintf(`
		<h2>Здравствуйте, %s!</h2>
		<p>Был получен запрос на сброс пароля для вашей учетной записи.</p>
		<p>Чтобы установить новый пароль, перейдите по ссылке ниже:</p>
		<p><a href="%s" style="display:inline-block;padding:10px 20px;background-color:#228be6;color:#ffffff;text-decoration:none;border-radius:4px;">Сбросить пароль</a></p>
		<p>Или скопируйте ссылку в адресную строку браузера: <br/><code>%s</code></p>
		<p>Ссылка действительна в течение 1 часа. Если вы не запрашивали сброс пароля, просто проигнорируйте это письмо.</p>
	`, username, link, link)
	textBody := fmt.Sprintf("Здравствуйте, %s!\n\nДля сброса пароля перейдите по ссылке:\n%s\n\nСсылка действительна 1 час.", username, link)

	return s.send(ctx, toEmail, subject, htmlBody, textBody)
}

func (s *ResendEmailService) SendEmailChangeVerification(ctx context.Context, toNewEmail, username, token string) error {
	link := fmt.Sprintf("%s/auth/confirm-email-change?token=%s", s.AppBaseURL, token)
	subject := "Подтверждение смены адреса электронной почты на платформе Gate"
	htmlBody := fmt.Sprintf(`
		<h2>Здравствуйте, %s!</h2>
		<p>Вы запросили смену адреса электронной почты для вашей учетной записи на этот адрес (%s).</p>
		<p>Для подтверждения нового адреса перейдите по ссылке ниже:</p>
		<p><a href="%s" style="display:inline-block;padding:10px 20px;background-color:#228be6;color:#ffffff;text-decoration:none;border-radius:4px;">Подтвердить новый email</a></p>
		<p>Или скопируйте ссылку в адресную строку браузера: <br/><code>%s</code></p>
		<p>Ссылка действительна в течение 24 часов.</p>
	`, username, toNewEmail, link, link)
	textBody := fmt.Sprintf("Здравствуйте, %s!\n\nДля подтверждения нового email перейдите по ссылке:\n%s\n\nСсылка действительна 24 часа.", username, link)

	return s.send(ctx, toNewEmail, subject, htmlBody, textBody)
}

func (s *ResendEmailService) SendEmailChangeAlert(ctx context.Context, toOldEmail, username, newEmail string) error {
	subject := "Уведомление о запросе смены email на платформе Gate"
	htmlBody := fmt.Sprintf(`
		<h2>Здравствуйте, %s!</h2>
		<p>Для вашей учетной записи на платформе Gate был создан запрос на изменение адреса электронной почты на <strong>%s</strong>.</p>
		<p>До момента подтверждения по ссылке из письма ваш текущий адрес остается активным.</p>
		<p><strong>Если это были не вы</strong>, немедленно войдите в систему и смените пароль в настройках профиля.</p>
	`, username, newEmail)
	textBody := fmt.Sprintf("Здравствуйте, %s!\n\nДля вашей учетной записи был создан запрос на смену email на %s.\nЕсли это были не вы, немедленно смените пароль.", username, newEmail)

	return s.send(ctx, toOldEmail, subject, htmlBody, textBody)
}

func (s *ResendEmailService) SendOrgInvitationEmail(ctx context.Context, toEmail, username, orgName, orgLogin, role, inviterUsername string) error {
	link := fmt.Sprintf("%s/notifications", s.AppBaseURL)
	subject := fmt.Sprintf("Приглашение в организацию %s на платформе Gate", orgName)
	htmlBody := fmt.Sprintf(`
		<h2>Здравствуйте, %s!</h2>
		<p>Пользователь <strong>@%s</strong> пригласил вас вступить в организацию <strong>%s</strong> (роль: <strong>%s</strong>).</p>
		<p>Вы можете принять или отклонить это приглашение в разделе уведомлений:</p>
		<p><a href="%s" style="display:inline-block;padding:10px 20px;background-color:#228be6;color:#ffffff;text-decoration:none;border-radius:4px;">Перейти к уведомлениям</a></p>
	`, username, inviterUsername, orgName, role, link)
	textBody := fmt.Sprintf("Здравствуйте, %s!\n\nПользователь @%s пригласил вас в организацию %s (роль: %s).\nПринять приглашение: %s", username, inviterUsername, orgName, role, link)

	return s.send(ctx, toEmail, subject, htmlBody, textBody)
}

func (s *ResendEmailService) SendOrgJoinRequestEmail(ctx context.Context, toEmail, adminUsername, applicantUsername, orgName, orgLogin string) error {
	link := fmt.Sprintf("%s/%s/settings/members", s.AppBaseURL, orgLogin)
	subject := fmt.Sprintf("Новая заявка на вступление в организацию %s", orgName)
	htmlBody := fmt.Sprintf(`
		<h2>Здравствуйте, %s!</h2>
		<p>Пользователь <strong>@%s</strong> подал заявку на вступление в вашу организацию <strong>%s</strong>.</p>
		<p>Вы можете рассмотреть заявку в панели управления организацией:</p>
		<p><a href="%s" style="display:inline-block;padding:10px 20px;background-color:#228be6;color:#ffffff;text-decoration:none;border-radius:4px;">Перейти к заявкам</a></p>
	`, adminUsername, applicantUsername, orgName, link)
	textBody := fmt.Sprintf("Здравствуйте, %s!\n\nПользователь @%s подал заявку на вступление в организацию %s.\nРассмотреть заявку: %s", adminUsername, applicantUsername, orgName, link)

	return s.send(ctx, toEmail, subject, htmlBody, textBody)
}

func (s *ResendEmailService) SendOrgJoinRequestResolvedEmail(ctx context.Context, toEmail, username, orgName, orgLogin string, approved bool) error {
	link := fmt.Sprintf("%s/%s", s.AppBaseURL, orgLogin)
	statusText := "одобрена"
	if !approved {
		statusText = "отклонена"
	}
	subject := fmt.Sprintf("Ваша заявка в организацию %s %s", orgName, statusText)
	htmlBody := fmt.Sprintf(`
		<h2>Здравствуйте, %s!</h2>
		<p>Ваша заявка на вступление в организацию <strong>%s</strong> была <strong>%s</strong>.</p>
		<p><a href="%s" style="display:inline-block;padding:10px 20px;background-color:#228be6;color:#ffffff;text-decoration:none;border-radius:4px;">Перейти на страницу организации</a></p>
	`, username, orgName, statusText, link)
	textBody := fmt.Sprintf("Здравствуйте, %s!\n\nВаша заявка на вступление в организацию %s была %s.\nСтраница организации: %s", username, orgName, statusText, link)

	return s.send(ctx, toEmail, subject, htmlBody, textBody)
}

func (s *ResendEmailService) SendContestJoinRequestEmail(ctx context.Context, toEmail, moderatorUsername, applicantUsername, contestTitle, orgLogin, contestLogin string) error {
	link := fmt.Sprintf("%s/%s/%s", s.AppBaseURL, orgLogin, contestLogin)
	subject := fmt.Sprintf("Новая заявка на участие в контесте %s", contestTitle)
	htmlBody := fmt.Sprintf(`
		<h2>Здравствуйте, %s!</h2>
		<p>Пользователь <strong>@%s</strong> запросил доступ к участию в контесте <strong>%s</strong>.</p>
		<p><a href="%s" style="display:inline-block;padding:10px 20px;background-color:#228be6;color:#ffffff;text-decoration:none;border-radius:4px;">Перейти к контесту</a></p>
	`, moderatorUsername, applicantUsername, contestTitle, link)
	textBody := fmt.Sprintf("Здравствуйте, %s!\n\nПользователь @%s запросил доступ к контесту %s.\nСсылка: %s", moderatorUsername, applicantUsername, contestTitle, link)

	return s.send(ctx, toEmail, subject, htmlBody, textBody)
}

func (s *ResendEmailService) SendContestJoinRequestResolvedEmail(ctx context.Context, toEmail, username, contestTitle, orgLogin, contestLogin string, approved bool) error {
	link := fmt.Sprintf("%s/%s/%s", s.AppBaseURL, orgLogin, contestLogin)
	statusText := "одобрена"
	if !approved {
		statusText = "отклонена"
	}
	subject := fmt.Sprintf("Ваша заявка на контест %s %s", contestTitle, statusText)
	htmlBody := fmt.Sprintf(`
		<h2>Здравствуйте, %s!</h2>
		<p>Ваша заявка на участие в контесте <strong>%s</strong> была <strong>%s</strong>.</p>
		<p><a href="%s" style="display:inline-block;padding:10px 20px;background-color:#228be6;color:#ffffff;text-decoration:none;border-radius:4px;">Перейти к контесту</a></p>
	`, username, contestTitle, statusText, link)
	textBody := fmt.Sprintf("Здравствуйте, %s!\n\nВаша заявка на участие в контесте %s была %s.\nСсылка: %s", username, contestTitle, statusText, link)

	return s.send(ctx, toEmail, subject, htmlBody, textBody)
}
