package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/ricki/codexsess/internal/store"
)

type messagingChannelSettings struct {
	Telegram telegramChannelConfig `json:"telegram"`
	Discord  discordChannelConfig  `json:"discord"`
	WhatsApp whatsappChannelConfig `json:"whatsapp"`
}

type telegramChannelConfig struct {
	Enabled     bool   `json:"enabled"`
	BotToken    string `json:"bot_token"`
	SecretToken string `json:"secret_token"`
	Reply       bool   `json:"reply_enabled"`
	Model       string `json:"model"`
}

type discordChannelConfig struct {
	Enabled       bool   `json:"enabled"`
	InboundSecret string `json:"inbound_secret"`
	WebhookURL    string `json:"webhook_url"`
	Reply         bool   `json:"reply_enabled"`
	Model         string `json:"model"`
}

type whatsappChannelConfig struct {
	Enabled       bool   `json:"enabled"`
	VerifyToken   string `json:"verify_token"`
	AccessToken   string `json:"access_token"`
	PhoneNumberID string `json:"phone_number_id"`
	Reply         bool   `json:"reply_enabled"`
	Model         string `json:"model"`
}

type channelChatTurn struct {
	Role    string
	Content string
}

type channelPairLink struct {
	Channel       string `json:"channel"`
	UserID        string `json:"user_id"`
	SessionID     string `json:"session_id"`
	ModelOverride string `json:"model_override,omitempty"`
	PairedAt      string `json:"paired_at"`
}

type channelPairRequest struct {
	Channel   string `json:"channel"`
	UserID    string `json:"user_id"`
	Code      string `json:"code"`
	ExpiresAt string `json:"expires_at"`
}

func defaultMessagingChannelSettings() messagingChannelSettings {
	return messagingChannelSettings{
		Telegram: telegramChannelConfig{
			Enabled: false,
			Reply:   true,
			Model:   "gpt-5.2-codex",
		},
		Discord: discordChannelConfig{
			Enabled: false,
			Reply:   true,
			Model:   "gpt-5.2-codex",
		},
		WhatsApp: whatsappChannelConfig{
			Enabled: false,
			Reply:   true,
			Model:   "gpt-5.2-codex",
		},
	}
}

func (s *Server) currentMessagingChannelSettings(ctx context.Context) messagingChannelSettings {
	cfg := defaultMessagingChannelSettings()
	if s == nil || s.svc == nil || s.svc.Store == nil {
		return cfg
	}
	cfg.Telegram.Enabled = s.readBoolSetting(ctx, store.SettingTelegramEnabled, cfg.Telegram.Enabled)
	cfg.Telegram.BotToken = strings.TrimSpace(s.readStringSetting(ctx, store.SettingTelegramBotToken, cfg.Telegram.BotToken))
	cfg.Telegram.SecretToken = strings.TrimSpace(s.readStringSetting(ctx, store.SettingTelegramSecretToken, cfg.Telegram.SecretToken))
	cfg.Telegram.Reply = s.readBoolSetting(ctx, store.SettingTelegramReplyEnabled, cfg.Telegram.Reply)
	cfg.Telegram.Model = firstNonEmpty(strings.TrimSpace(s.readStringSetting(ctx, store.SettingTelegramModel, cfg.Telegram.Model)), cfg.Telegram.Model)

	cfg.Discord.Enabled = s.readBoolSetting(ctx, store.SettingDiscordEnabled, cfg.Discord.Enabled)
	cfg.Discord.InboundSecret = strings.TrimSpace(s.readStringSetting(ctx, store.SettingDiscordInboundSecret, cfg.Discord.InboundSecret))
	cfg.Discord.WebhookURL = strings.TrimSpace(s.readStringSetting(ctx, store.SettingDiscordWebhookURL, cfg.Discord.WebhookURL))
	cfg.Discord.Reply = s.readBoolSetting(ctx, store.SettingDiscordReplyEnabled, cfg.Discord.Reply)
	cfg.Discord.Model = firstNonEmpty(strings.TrimSpace(s.readStringSetting(ctx, store.SettingDiscordModel, cfg.Discord.Model)), cfg.Discord.Model)

	cfg.WhatsApp.Enabled = s.readBoolSetting(ctx, store.SettingWhatsAppEnabled, cfg.WhatsApp.Enabled)
	cfg.WhatsApp.VerifyToken = strings.TrimSpace(s.readStringSetting(ctx, store.SettingWhatsAppVerifyToken, cfg.WhatsApp.VerifyToken))
	cfg.WhatsApp.AccessToken = strings.TrimSpace(s.readStringSetting(ctx, store.SettingWhatsAppAccessToken, cfg.WhatsApp.AccessToken))
	cfg.WhatsApp.PhoneNumberID = strings.TrimSpace(s.readStringSetting(ctx, store.SettingWhatsAppPhoneNumberID, cfg.WhatsApp.PhoneNumberID))
	cfg.WhatsApp.Reply = s.readBoolSetting(ctx, store.SettingWhatsAppReplyEnabled, cfg.WhatsApp.Reply)
	cfg.WhatsApp.Model = firstNonEmpty(strings.TrimSpace(s.readStringSetting(ctx, store.SettingWhatsAppModel, cfg.WhatsApp.Model)), cfg.WhatsApp.Model)
	return cfg
}

func (s *Server) readStringSetting(ctx context.Context, key string, fallback string) string {
	if s == nil || s.svc == nil || s.svc.Store == nil {
		return fallback
	}
	v, ok, err := s.svc.Store.MustGetSetting(ctx, key)
	if err != nil || !ok {
		return fallback
	}
	return v
}

func (s *Server) readBoolSetting(ctx context.Context, key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(s.readStringSetting(ctx, key, "")))
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func (s *Server) saveMessagingChannelSettings(ctx context.Context, cfg messagingChannelSettings) error {
	pairs := map[string]string{
		store.SettingTelegramEnabled:       boolString(cfg.Telegram.Enabled),
		store.SettingTelegramBotToken:      strings.TrimSpace(cfg.Telegram.BotToken),
		store.SettingTelegramSecretToken:   strings.TrimSpace(cfg.Telegram.SecretToken),
		store.SettingTelegramReplyEnabled:  boolString(cfg.Telegram.Reply),
		store.SettingTelegramModel:         strings.TrimSpace(cfg.Telegram.Model),
		store.SettingDiscordEnabled:        boolString(cfg.Discord.Enabled),
		store.SettingDiscordInboundSecret:  strings.TrimSpace(cfg.Discord.InboundSecret),
		store.SettingDiscordWebhookURL:     strings.TrimSpace(cfg.Discord.WebhookURL),
		store.SettingDiscordReplyEnabled:   boolString(cfg.Discord.Reply),
		store.SettingDiscordModel:          strings.TrimSpace(cfg.Discord.Model),
		store.SettingWhatsAppEnabled:       boolString(cfg.WhatsApp.Enabled),
		store.SettingWhatsAppVerifyToken:   strings.TrimSpace(cfg.WhatsApp.VerifyToken),
		store.SettingWhatsAppAccessToken:   strings.TrimSpace(cfg.WhatsApp.AccessToken),
		store.SettingWhatsAppPhoneNumberID: strings.TrimSpace(cfg.WhatsApp.PhoneNumberID),
		store.SettingWhatsAppReplyEnabled:  boolString(cfg.WhatsApp.Reply),
		store.SettingWhatsAppModel:         strings.TrimSpace(cfg.WhatsApp.Model),
	}
	for key, val := range pairs {
		if err := s.saveSetting(ctx, key, val); err != nil {
			return err
		}
	}
	return nil
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func normalizeChannelModel(v string) string {
	model := strings.TrimSpace(v)
	if model == "" {
		return "gpt-5.2-codex"
	}
	return model
}

func trimReply(text string) string {
	x := strings.TrimSpace(text)
	if x == "" {
		return "OK"
	}
	if len(x) > 3500 {
		return x[:3500]
	}
	return x
}

func (s *Server) channelReply(ctx context.Context, channel, userID, model, text string) string {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return "Pesan kosong."
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "/start") {
		if link, ok := s.channelPairLink(ctx, channel, userID); ok {
			return fmt.Sprintf("Connected. Session aktif: %s. Buka web: /chat?id=%s", link.SessionID, link.SessionID)
		}
		return "Connected. Kirim /pair untuk generate pair code, lalu approve dari dashboard."
	}
	if strings.HasPrefix(lower, "/ping") {
		return "pong"
	}
	if strings.HasPrefix(lower, "/pair") {
		req := s.issueChannelPairRequest(channel, userID)
		return fmt.Sprintf("Pair code: %s (expired: %s). Approve via dashboard > Settings > Channel Pairing.", req.Code, req.ExpiresAt)
	}
	if strings.HasPrefix(lower, "/session") {
		if link, ok := s.channelPairLink(ctx, channel, userID); ok {
			return fmt.Sprintf("Session ID: %s. Web: /chat?id=%s", link.SessionID, link.SessionID)
		}
		return "Belum paired. Kirim /pair dulu."
	}
	link, ok := s.channelPairLink(ctx, channel, userID)
	if !ok {
		return "Akun channel belum paired. Kirim /pair lalu approve dari dashboard."
	}
	if strings.HasPrefix(lower, "/model") {
		fields := strings.Fields(raw)
		if len(fields) == 1 {
			active := firstNonEmpty(strings.TrimSpace(link.ModelOverride), normalizeChannelModel(model))
			return "Model aktif: " + active
		}
		newModel := normalizeChannelModel(strings.Join(fields[1:], " "))
		s.setChannelPairModel(ctx, channel, userID, newModel)
		return fmt.Sprintf("Model session di-set ke %s", newModel)
	}
	modelToUse := firstNonEmpty(strings.TrimSpace(link.ModelOverride), normalizeChannelModel(model))
	reply, err := s.runSessionChatCompletion(ctx, link.SessionID, modelToUse, raw)
	if err != nil {
		return "Gagal proses pesan: " + err.Error()
	}
	return trimReply(reply)
}

func (s *Server) runSessionChatCompletion(ctx context.Context, sessionID, model, text string) (string, error) {
	sid := strings.TrimSpace(sessionID)
	if sid == "" {
		return "", fmt.Errorf("session id is empty")
	}
	result, err := s.svc.SendCodingMessage(ctx, sid, strings.TrimSpace(text), normalizeChannelModel(model), "medium", "", "", "chat")
	if err != nil {
		return "", err
	}
	if x := strings.TrimSpace(result.Assistant.Content); x != "" {
		return x, nil
	}
	for i := len(result.Assistants) - 1; i >= 0; i-- {
		if x := strings.TrimSpace(result.Assistants[i].Content); x != "" {
			return x, nil
		}
	}
	return "", fmt.Errorf("no assistant response")
}

func (s *Server) runLocalChatCompletion(ctx context.Context, model string, messages []map[string]string) (string, error) {
	base := strings.TrimRight(s.localBaseURL(), "/")
	payload := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   false,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(s.currentAPIKey()))
	client := &http.Client{Timeout: 55 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = res.Body.Close() }()
	var body struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error any `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return "", fmt.Errorf("upstream status %d", res.StatusCode)
	}
	if len(body.Choices) == 0 {
		return "", fmt.Errorf("no reply")
	}
	return strings.TrimSpace(body.Choices[0].Message.Content), nil
}

func (s *Server) channelConversationMessages(channel, userID, text string) []map[string]string {
	msgs := []map[string]string{
		{"role": "system", "content": "You are a concise assistant for chat channels. Keep answers short and practical."},
	}
	key := strings.TrimSpace(channel) + ":" + strings.TrimSpace(userID)
	s.channelConvMu.Lock()
	history := append([]channelChatTurn(nil), s.channelConversations[key]...)
	s.channelConvMu.Unlock()
	if len(history) > 10 {
		history = history[len(history)-10:]
	}
	for _, turn := range history {
		role := strings.TrimSpace(strings.ToLower(turn.Role))
		content := strings.TrimSpace(turn.Content)
		if content == "" {
			continue
		}
		if role != "assistant" {
			role = "user"
		}
		msgs = append(msgs, map[string]string{"role": role, "content": content})
	}
	msgs = append(msgs, map[string]string{"role": "user", "content": strings.TrimSpace(text)})
	return msgs
}

func (s *Server) appendChannelConversation(channel, userID, userText, replyText string) {
	key := strings.TrimSpace(channel) + ":" + strings.TrimSpace(userID)
	s.channelConvMu.Lock()
	defer s.channelConvMu.Unlock()
	history := append(s.channelConversations[key],
		channelChatTurn{Role: "user", Content: strings.TrimSpace(userText)},
		channelChatTurn{Role: "assistant", Content: strings.TrimSpace(replyText)},
	)
	if len(history) > 20 {
		history = history[len(history)-20:]
	}
	s.channelConversations[key] = history
}

func (s *Server) localBaseURL() string {
	host := strings.TrimSpace(s.bindAddr)
	if host == "" {
		return "http://127.0.0.1:3061"
	}
	if strings.HasPrefix(host, "0.0.0.0:") {
		host = strings.Replace(host, "0.0.0.0:", "127.0.0.1:", 1)
	}
	if strings.HasPrefix(host, ":") {
		host = "127.0.0.1" + host
	}
	return "http://" + host
}

func (s *Server) handleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	cfg := s.currentMessagingChannelSettings(r.Context()).Telegram
	if !cfg.Enabled {
		respondJSON(w, http.StatusOK, map[string]any{"ok": true, "ignored": "disabled"})
		return
	}
	if cfg.SecretToken != "" && strings.TrimSpace(r.Header.Get("X-Telegram-Bot-Api-Secret-Token")) != cfg.SecretToken {
		respondErr(w, http.StatusUnauthorized, "unauthorized", "invalid telegram secret token")
		return
	}
	var update struct {
		Message *struct {
			Text string `json:"text"`
			Chat struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		respondErr(w, http.StatusBadRequest, "bad_request", "invalid telegram update payload")
		return
	}
	if update.Message == nil || strings.TrimSpace(update.Message.Text) == "" {
		respondJSON(w, http.StatusOK, map[string]any{"ok": true, "ignored": "no_message"})
		return
	}
	if !cfg.Reply {
		respondJSON(w, http.StatusOK, map[string]any{"ok": true, "received": true})
		return
	}
	reply := s.channelReply(r.Context(), "telegram", fmt.Sprintf("%d", update.Message.Chat.ID), cfg.Model, update.Message.Text)
	if err := s.sendTelegramMessage(r.Context(), cfg.BotToken, update.Message.Chat.ID, reply); err != nil {
		respondErr(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) sendTelegramMessage(ctx context.Context, botToken string, chatID int64, text string) error {
	token := strings.TrimSpace(botToken)
	if token == "" {
		return fmt.Errorf("telegram bot token is empty")
	}
	payload := map[string]any{
		"chat_id": chatID,
		"text":    trimReply(text),
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+token+"/sendMessage", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("telegram send failed status=%d", res.StatusCode)
	}
	return nil
}

func (s *Server) runTelegramPollingLoop(ctx context.Context) {
	backoff := 2 * time.Second
	webhookCleared := false
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		cfg := s.currentMessagingChannelSettings(context.Background()).Telegram
		if !cfg.Enabled || strings.TrimSpace(cfg.BotToken) == "" {
			webhookCleared = false
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
				continue
			}
		}
		if !webhookCleared {
			if err := s.deleteTelegramWebhook(ctx, cfg.BotToken); err != nil {
				log.Printf("[telegram-polling] delete webhook failed: %v", err)
			}
			webhookCleared = true
			select {
			case <-ctx.Done():
				return
			case <-time.After(250 * time.Millisecond):
			}
		}

		if err := s.telegramPollOnce(ctx, cfg); err != nil {
			log.Printf("[telegram-polling] poll failed: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > 20*time.Second {
				backoff = 20 * time.Second
			}
			continue
		}
		backoff = 2 * time.Second
	}
}

func (s *Server) telegramPollOnce(ctx context.Context, cfg telegramChannelConfig) error {
	offset := s.getTelegramUpdateOffset()
	payload := map[string]any{
		"timeout":         30,
		"offset":          offset,
		"allowed_updates": []string{"message"},
	}
	raw, _ := json.Marshal(payload)
	endpoint := "https://api.telegram.org/bot" + strings.TrimSpace(cfg.BotToken) + "/getUpdates"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 40 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	bodyRaw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("getUpdates status=%d body=%s", res.StatusCode, strings.TrimSpace(string(bodyRaw)))
	}
	var body struct {
		OK          bool `json:"ok"`
		Description string
		Result      []struct {
			UpdateID int64 `json:"update_id"`
			Message  *struct {
				Text string `json:"text"`
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
				From struct {
					ID    int64 `json:"id"`
					IsBot bool  `json:"is_bot"`
				} `json:"from"`
			} `json:"message"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bodyRaw, &body); err != nil {
		return err
	}
	if !body.OK {
		return fmt.Errorf("getUpdates not ok: %s", strings.TrimSpace(body.Description))
	}
	for _, upd := range body.Result {
		nextOffset := upd.UpdateID + 1
		s.setTelegramUpdateOffset(nextOffset)
		if upd.Message == nil || upd.Message.From.IsBot {
			continue
		}
		if !cfg.Reply {
			continue
		}
		userText := strings.TrimSpace(upd.Message.Text)
		if userText == "" {
			continue
		}
		userID := fmt.Sprintf("%d", upd.Message.Chat.ID)
		reply := s.channelReply(ctx, "telegram", userID, cfg.Model, userText)
		if err := s.sendTelegramMessage(ctx, cfg.BotToken, upd.Message.Chat.ID, reply); err != nil {
			log.Printf("[telegram-polling] send message failed: %v", err)
		}
	}
	return nil
}

func (s *Server) deleteTelegramWebhook(ctx context.Context, botToken string) error {
	token := strings.TrimSpace(botToken)
	if token == "" {
		return fmt.Errorf("telegram bot token is empty")
	}
	payload := map[string]any{
		"drop_pending_updates": false,
	}
	raw, _ := json.Marshal(payload)
	endpoint := "https://api.telegram.org/bot" + token + "/deleteWebhook"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("deleteWebhook status=%d body=%s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (s *Server) getTelegramUpdateOffset() int64 {
	s.telegramOffsetMu.Lock()
	defer s.telegramOffsetMu.Unlock()
	return s.telegramUpdateOffset
}

func (s *Server) setTelegramUpdateOffset(v int64) {
	s.telegramOffsetMu.Lock()
	if v > s.telegramUpdateOffset {
		s.telegramUpdateOffset = v
	}
	s.telegramOffsetMu.Unlock()
}

func channelPairKey(channel, userID string) string {
	return strings.ToLower(strings.TrimSpace(channel)) + ":" + strings.TrimSpace(userID)
}

func (s *Server) loadChannelPairLinks(ctx context.Context) {
	s.channelPairMu.Lock()
	already := len(s.channelPairLinks) > 0
	s.channelPairMu.Unlock()
	if already {
		return
	}
	raw, ok, err := s.svc.Store.MustGetSetting(ctx, store.SettingChannelPairLinks)
	if err != nil || !ok || strings.TrimSpace(raw) == "" {
		return
	}
	var links []channelPairLink
	if err := json.Unmarshal([]byte(raw), &links); err != nil {
		return
	}
	tmp := map[string]channelPairLink{}
	for _, item := range links {
		ch := strings.ToLower(strings.TrimSpace(item.Channel))
		uid := strings.TrimSpace(item.UserID)
		sid := strings.TrimSpace(item.SessionID)
		if ch == "" || uid == "" || sid == "" {
			continue
		}
		item.Channel = ch
		item.UserID = uid
		item.SessionID = sid
		tmp[channelPairKey(ch, uid)] = item
	}
	s.channelPairMu.Lock()
	if len(s.channelPairLinks) == 0 {
		s.channelPairLinks = tmp
	}
	s.channelPairMu.Unlock()
}

func (s *Server) persistChannelPairLinks(ctx context.Context) {
	s.channelPairMu.Lock()
	links := make([]channelPairLink, 0, len(s.channelPairLinks))
	for _, item := range s.channelPairLinks {
		links = append(links, item)
	}
	s.channelPairMu.Unlock()
	sort.Slice(links, func(i, j int) bool {
		a := links[i]
		b := links[j]
		if a.Channel != b.Channel {
			return a.Channel < b.Channel
		}
		if a.UserID != b.UserID {
			return a.UserID < b.UserID
		}
		return a.SessionID < b.SessionID
	})
	raw, _ := json.Marshal(links)
	_ = s.saveSetting(ctx, store.SettingChannelPairLinks, string(raw))
}

func (s *Server) channelPairLink(ctx context.Context, channel, userID string) (channelPairLink, bool) {
	s.loadChannelPairLinks(ctx)
	key := channelPairKey(channel, userID)
	s.channelPairMu.Lock()
	item, ok := s.channelPairLinks[key]
	s.channelPairMu.Unlock()
	if !ok {
		return channelPairLink{}, false
	}
	return item, true
}

func (s *Server) setChannelPairModel(ctx context.Context, channel, userID, model string) {
	key := channelPairKey(channel, userID)
	s.loadChannelPairLinks(ctx)
	s.channelPairMu.Lock()
	item, ok := s.channelPairLinks[key]
	if ok {
		item.ModelOverride = normalizeChannelModel(model)
		s.channelPairLinks[key] = item
	}
	s.channelPairMu.Unlock()
	if ok {
		s.persistChannelPairLinks(ctx)
	}
}

func generatePairCode() string {
	v, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		n := time.Now().UnixNano() % 900000
		return fmt.Sprintf("%06d", n+100000)
	}
	return fmt.Sprintf("%06d", v.Int64()+100000)
}

func (s *Server) issueChannelPairRequest(channel, userID string) channelPairRequest {
	now := time.Now().UTC()
	item := channelPairRequest{
		Channel:   strings.ToLower(strings.TrimSpace(channel)),
		UserID:    strings.TrimSpace(userID),
		Code:      generatePairCode(),
		ExpiresAt: now.Add(10 * time.Minute).Format(time.RFC3339),
	}
	key := channelPairKey(item.Channel, item.UserID)
	s.channelPairMu.Lock()
	if s.channelPairRequests == nil {
		s.channelPairRequests = map[string]channelPairRequest{}
	}
	s.channelPairRequests[key] = item
	s.channelPairMu.Unlock()
	return item
}

func (s *Server) currentPairingState(ctx context.Context) (pending []channelPairRequest, links []channelPairLink) {
	s.loadChannelPairLinks(ctx)
	now := time.Now().UTC()
	s.channelPairMu.Lock()
	for key, item := range s.channelPairRequests {
		exp, err := time.Parse(time.RFC3339, strings.TrimSpace(item.ExpiresAt))
		if err != nil || now.After(exp) {
			delete(s.channelPairRequests, key)
			continue
		}
		pending = append(pending, item)
	}
	for _, item := range s.channelPairLinks {
		links = append(links, item)
	}
	s.channelPairMu.Unlock()
	sort.Slice(pending, func(i, j int) bool {
		if pending[i].Channel != pending[j].Channel {
			return pending[i].Channel < pending[j].Channel
		}
		return pending[i].UserID < pending[j].UserID
	})
	sort.Slice(links, func(i, j int) bool {
		if links[i].Channel != links[j].Channel {
			return links[i].Channel < links[j].Channel
		}
		return links[i].UserID < links[j].UserID
	})
	return pending, links
}

func (s *Server) approvePairing(ctx context.Context, channel, userID, code, sessionID string) (channelPairLink, error) {
	ch := strings.ToLower(strings.TrimSpace(channel))
	uid := strings.TrimSpace(userID)
	pairCode := strings.TrimSpace(code)
	if ch == "" || uid == "" || pairCode == "" {
		return channelPairLink{}, fmt.Errorf("channel, user_id, code wajib")
	}
	key := channelPairKey(ch, uid)
	s.channelPairMu.Lock()
	req, ok := s.channelPairRequests[key]
	s.channelPairMu.Unlock()
	if !ok {
		return channelPairLink{}, fmt.Errorf("pair request tidak ditemukan")
	}
	if req.Code != pairCode {
		return channelPairLink{}, fmt.Errorf("pair code tidak cocok")
	}
	exp, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ExpiresAt))
	if err != nil || time.Now().UTC().After(exp) {
		s.channelPairMu.Lock()
		delete(s.channelPairRequests, key)
		s.channelPairMu.Unlock()
		return channelPairLink{}, fmt.Errorf("pair code expired")
	}

	sid := strings.TrimSpace(sessionID)
	if sid == "" {
		model := "gpt-5.2-codex"
		cfg := s.currentMessagingChannelSettings(ctx)
		switch ch {
		case "telegram":
			model = normalizeChannelModel(cfg.Telegram.Model)
		case "discord":
			model = normalizeChannelModel(cfg.Discord.Model)
		case "whatsapp":
			model = normalizeChannelModel(cfg.WhatsApp.Model)
		}
		created, err := s.svc.CreateCodingSession(ctx, fmt.Sprintf("%s:%s", ch, uid), model, "medium", "~/", "write")
		if err != nil {
			return channelPairLink{}, err
		}
		sid = strings.TrimSpace(created.ID)
	}

	link := channelPairLink{
		Channel:   ch,
		UserID:    uid,
		SessionID: sid,
		PairedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	s.loadChannelPairLinks(ctx)
	s.channelPairMu.Lock()
	if s.channelPairLinks == nil {
		s.channelPairLinks = map[string]channelPairLink{}
	}
	s.channelPairLinks[key] = link
	delete(s.channelPairRequests, key)
	s.channelPairMu.Unlock()
	s.persistChannelPairLinks(ctx)
	return link, nil
}

func (s *Server) revokePairing(ctx context.Context, channel, userID string) {
	ch := strings.ToLower(strings.TrimSpace(channel))
	uid := strings.TrimSpace(userID)
	if ch == "" || uid == "" {
		return
	}
	key := channelPairKey(ch, uid)
	s.loadChannelPairLinks(ctx)
	s.channelPairMu.Lock()
	delete(s.channelPairLinks, key)
	delete(s.channelPairRequests, key)
	s.channelPairMu.Unlock()
	s.persistChannelPairLinks(ctx)
}

func (s *Server) handleWebChannelPairing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	pending, links := s.currentPairingState(r.Context())
	respondJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"pending": pending,
		"links":   links,
	})
}

func (s *Server) handleWebChannelPairingApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req struct {
		Channel   string `json:"channel"`
		UserID    string `json:"user_id"`
		Code      string `json:"code"`
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErr(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	link, err := s.approvePairing(r.Context(), req.Channel, req.UserID, req.Code, req.SessionID)
	if err != nil {
		respondErr(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"link": link,
	})
}

func (s *Server) handleWebChannelPairingRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req struct {
		Channel string `json:"channel"`
		UserID  string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErr(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	s.revokePairing(r.Context(), req.Channel, req.UserID)
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleTelegramWebhookRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	cfg := s.currentMessagingChannelSettings(r.Context()).Telegram
	if strings.TrimSpace(cfg.BotToken) == "" {
		respondErr(w, http.StatusBadRequest, "bad_request", "telegram bot token is empty")
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	webhookURL := strings.TrimSpace(req.URL)
	if webhookURL == "" {
		base := strings.TrimRight(externalBaseURLFromRequest(r, s.bindAddr), "/")
		webhookURL = base + "/api/channels/telegram/webhook"
	}
	payload := map[string]any{
		"url": webhookURL,
	}
	if strings.TrimSpace(cfg.SecretToken) != "" {
		payload["secret_token"] = strings.TrimSpace(cfg.SecretToken)
	}
	raw, _ := json.Marshal(payload)
	endpoint := "https://api.telegram.org/bot" + strings.TrimSpace(cfg.BotToken) + "/setWebhook"
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(httpReq)
	if err != nil {
		respondErr(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	defer func() { _ = res.Body.Close() }()
	bodyRaw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode > 299 {
		respondErr(w, http.StatusBadGateway, "upstream_error", fmt.Sprintf("telegram setWebhook failed status=%d body=%s", res.StatusCode, strings.TrimSpace(string(bodyRaw))))
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"webhook_url": webhookURL,
		"telegram":    json.RawMessage(bodyRaw),
	})
}

func (s *Server) handleTelegramWebhookInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	cfg := s.currentMessagingChannelSettings(r.Context()).Telegram
	if strings.TrimSpace(cfg.BotToken) == "" {
		respondErr(w, http.StatusBadRequest, "bad_request", "telegram bot token is empty")
		return
	}
	endpoint := "https://api.telegram.org/bot" + strings.TrimSpace(cfg.BotToken) + "/getWebhookInfo"
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(httpReq)
	if err != nil {
		respondErr(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	defer func() { _ = res.Body.Close() }()
	bodyRaw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode > 299 {
		respondErr(w, http.StatusBadGateway, "upstream_error", fmt.Sprintf("telegram getWebhookInfo failed status=%d body=%s", res.StatusCode, strings.TrimSpace(string(bodyRaw))))
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"telegram": json.RawMessage(bodyRaw),
	})
}

func (s *Server) handleDiscordWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	cfg := s.currentMessagingChannelSettings(r.Context()).Discord
	if !cfg.Enabled {
		respondJSON(w, http.StatusOK, map[string]any{"ok": true, "ignored": "disabled"})
		return
	}
	if cfg.InboundSecret != "" && strings.TrimSpace(r.Header.Get("X-Codexsess-Discord-Secret")) != cfg.InboundSecret {
		respondErr(w, http.StatusUnauthorized, "unauthorized", "invalid discord inbound secret")
		return
	}
	var payload struct {
		Text       string `json:"text"`
		WebhookURL string `json:"webhook_url"`
		SenderID   string `json:"sender_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondErr(w, http.StatusBadRequest, "bad_request", "invalid discord payload")
		return
	}
	text := strings.TrimSpace(payload.Text)
	if text == "" {
		respondJSON(w, http.StatusOK, map[string]any{"ok": true, "ignored": "no_text"})
		return
	}
	if !cfg.Reply {
		respondJSON(w, http.StatusOK, map[string]any{"ok": true, "received": true})
		return
	}
	reply := s.channelReply(r.Context(), "discord", firstNonEmpty(strings.TrimSpace(payload.SenderID), strings.TrimSpace(r.RemoteAddr)), cfg.Model, text)
	target := firstNonEmpty(strings.TrimSpace(payload.WebhookURL), strings.TrimSpace(cfg.WebhookURL))
	if target == "" {
		respondErr(w, http.StatusBadRequest, "bad_request", "discord webhook_url not configured")
		return
	}
	if err := s.sendDiscordMessage(r.Context(), target, reply); err != nil {
		respondErr(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) sendDiscordMessage(ctx context.Context, webhookURL, text string) error {
	payload := map[string]any{"content": trimReply(text)}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(webhookURL), bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("discord send failed status=%d", res.StatusCode)
	}
	return nil
}

func (s *Server) handleWhatsAppWebhook(w http.ResponseWriter, r *http.Request) {
	cfg := s.currentMessagingChannelSettings(r.Context()).WhatsApp
	switch r.Method {
	case http.MethodGet:
		mode := strings.TrimSpace(r.URL.Query().Get("hub.mode"))
		challenge := strings.TrimSpace(r.URL.Query().Get("hub.challenge"))
		verifyToken := strings.TrimSpace(r.URL.Query().Get("hub.verify_token"))
		if cfg.Enabled && mode == "subscribe" && cfg.VerifyToken != "" && verifyToken == cfg.VerifyToken {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(challenge))
			return
		}
		respondErr(w, http.StatusUnauthorized, "unauthorized", "verification failed")
		return
	case http.MethodPost:
		if !cfg.Enabled {
			respondJSON(w, http.StatusOK, map[string]any{"ok": true, "ignored": "disabled"})
			return
		}
		var event struct {
			Entry []struct {
				Changes []struct {
					Value struct {
						Messages []struct {
							From string `json:"from"`
							Text struct {
								Body string `json:"body"`
							} `json:"text"`
						} `json:"messages"`
					} `json:"value"`
				} `json:"changes"`
			} `json:"entry"`
		}
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			respondErr(w, http.StatusBadRequest, "bad_request", "invalid whatsapp payload")
			return
		}
		if !cfg.Reply {
			respondJSON(w, http.StatusOK, map[string]any{"ok": true, "received": true})
			return
		}
		for _, entry := range event.Entry {
			for _, change := range entry.Changes {
				for _, msg := range change.Value.Messages {
					userText := strings.TrimSpace(msg.Text.Body)
					if userText == "" {
						continue
					}
					reply := s.channelReply(r.Context(), "whatsapp", strings.TrimSpace(msg.From), cfg.Model, userText)
					if err := s.sendWhatsAppMessage(r.Context(), cfg.AccessToken, cfg.PhoneNumberID, strings.TrimSpace(msg.From), reply); err != nil {
						respondErr(w, http.StatusBadGateway, "upstream_error", err.Error())
						return
					}
				}
			}
		}
		respondJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	default:
		respondErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func (s *Server) sendWhatsAppMessage(ctx context.Context, accessToken, phoneNumberID, to, text string) error {
	token := strings.TrimSpace(accessToken)
	phoneID := strings.TrimSpace(phoneNumberID)
	target := strings.TrimSpace(to)
	if token == "" || phoneID == "" {
		return fmt.Errorf("whatsapp access token / phone number id not configured")
	}
	if target == "" {
		return fmt.Errorf("whatsapp target is empty")
	}
	payload := map[string]any{
		"messaging_product": "whatsapp",
		"to":                target,
		"type":              "text",
		"text": map[string]string{
			"body": trimReply(text),
		},
	}
	raw, _ := json.Marshal(payload)
	url := "https://graph.facebook.com/v19.0/" + phoneID + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("whatsapp send failed status=%d", res.StatusCode)
	}
	return nil
}
