-- Migration 004: Seed default notification templates

-- Welcome templates
INSERT INTO notification_templates (name, channel, subject, body, locale, version, active) VALUES
('welcome', 'ses', 'Welcome to {{.AppName}}, {{.UserName}}!',
 'Hi {{.UserName}},

Welcome to {{.AppName}}! Your account has been successfully created.

Get started by completing your profile and connecting with friends.

Best regards,
The {{.AppName}} Team',
 'en', 1, true),

('welcome', 'ses', '{{.AppName}}에 오신 것을 환영합니다, {{.UserName}}님!',
 '안녕하세요 {{.UserName}}님,

{{.AppName}}에 가입해 주셔서 감사합니다. 계정이 성공적으로 생성되었습니다.

프로필을 완성하고 친구들과 연결해보세요.

감사합니다,
{{.AppName}} 팀',
 'ko', 1, true),

('welcome', 'fcm', 'Welcome to {{.AppName}}!',
 'Hi {{.UserName}}, your account is ready. Start exploring now!',
 'en', 1, true),

('welcome', 'fcm', '{{.AppName}}에 오신 것을 환영합니다!',
 '{{.UserName}}님, 계정이 준비되었습니다. 지금 시작해보세요!',
 'ko', 1, true),

-- Password reset templates
('password_reset', 'ses', 'Reset your {{.AppName}} password',
 'Hi {{.UserName}},

We received a request to reset your password. Click the link below to create a new password:

{{.ResetLink}}

This link will expire in {{.ExpiryMinutes}} minutes.

If you did not request a password reset, please ignore this email or contact support.

Best regards,
The {{.AppName}} Team',
 'en', 1, true),

('password_reset', 'ses', '{{.AppName}} 비밀번호 재설정',
 '안녕하세요 {{.UserName}}님,

비밀번호 재설정 요청을 받았습니다. 아래 링크를 클릭하여 새 비밀번호를 설정하세요:

{{.ResetLink}}

이 링크는 {{.ExpiryMinutes}}분 후에 만료됩니다.

비밀번호 재설정을 요청하지 않으셨다면 이 이메일을 무시하시거나 고객센터에 문의해 주세요.

감사합니다,
{{.AppName}} 팀',
 'ko', 1, true),

('password_reset', 'sns', 'Your {{.AppName}} verification code is: {{.Code}}. Valid for {{.ExpiryMinutes}} minutes.',
 'Your {{.AppName}} verification code is: {{.Code}}. Valid for {{.ExpiryMinutes}} minutes.',
 'en', 1, true),

-- Friend request templates
('friend_request', 'fcm', 'New friend request',
 '{{.SenderName}} sent you a friend request.',
 'en', 1, true),

('friend_request', 'fcm', '새 친구 요청',
 '{{.SenderName}}님이 친구 요청을 보냈습니다.',
 'ko', 1, true),

('friend_request', 'in_app', 'New friend request',
 '{{.SenderName}} sent you a friend request.',
 'en', 1, true),

('friend_request', 'in_app', '새 친구 요청',
 '{{.SenderName}}님이 친구 요청을 보냈습니다.',
 'ko', 1, true),

-- Comment notification templates
('comment', 'fcm', 'New comment on your post',
 '{{.CommenterName}} commented on your post: "{{.CommentPreview}}"',
 'en', 1, true),

('comment', 'fcm', '내 게시물에 새 댓글',
 '{{.CommenterName}}님이 회원님의 게시물에 댓글을 달았습니다: "{{.CommentPreview}}"',
 'ko', 1, true),

('comment', 'in_app', 'New comment',
 '{{.CommenterName}} commented on your post: "{{.CommentPreview}}"',
 'en', 1, true),

('comment', 'in_app', '새 댓글',
 '{{.CommenterName}}님이 댓글을 달았습니다: "{{.CommentPreview}}"',
 'ko', 1, true),

-- Ad report templates
('ad_report', 'ses', 'Your ad campaign report is ready',
 'Hi {{.UserName}},

Your ad campaign "{{.CampaignName}}" performance report for {{.ReportDate}} is ready.

Summary:
- Impressions: {{.Impressions}}
- Clicks: {{.Clicks}}
- CTR: {{.CTR}}%
- Spend: ${{.Spend}}

View the full report at: {{.ReportLink}}

Best regards,
The {{.AppName}} Ads Team',
 'en', 1, true),

('ad_report', 'ses', '광고 캠페인 리포트가 준비되었습니다',
 '안녕하세요 {{.UserName}}님,

{{.ReportDate}} {{.CampaignName}} 광고 캠페인 성과 리포트가 준비되었습니다.

요약:
- 노출수: {{.Impressions}}
- 클릭수: {{.Clicks}}
- CTR: {{.CTR}}%
- 지출: ${{.Spend}}

전체 리포트 보기: {{.ReportLink}}

감사합니다,
{{.AppName}} 광고 팀',
 'ko', 1, true),

-- Like notification
('like', 'fcm', '{{.LikerName}} liked your post',
 '{{.LikerName}} liked your post.',
 'en', 1, true),

('like', 'in_app', '{{.LikerName}} liked your post',
 '{{.LikerName}} liked your post.',
 'en', 1, true),

-- Security alert
('security_alert', 'ses', 'Security alert for your {{.AppName}} account',
 'Hi {{.UserName}},

We detected a new login to your account from:
- Device: {{.Device}}
- Location: {{.Location}}
- Time: {{.LoginTime}}

If this was you, no action is needed. If you did not log in, please secure your account immediately.

Best regards,
The {{.AppName}} Security Team',
 'en', 1, true);
