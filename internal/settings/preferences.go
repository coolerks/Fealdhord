// Package settings 保存本地界面偏好，不保存文章或凭据。
package settings

import (
	"errors"
	"math"
	"regexp"

	"github.com/coolerks/Fealdhord/internal/agentrun"
)

type ScrollAnchor struct {
	ArticleID string  `json:"article_id"`
	Offset    float64 `json:"offset"`
}

type Preferences struct {
	DeveloperMode               bool                    `json:"developer_mode"`
	QuitOnWindowClose           bool                    `json:"quit_on_window_close"`
	RefreshIntervalMinutes      int                     `json:"refresh_interval_minutes"`
	NotificationMode            string                  `json:"notification_mode"`
	RefreshNotificationsEnabled bool                    `json:"refresh_notifications_enabled"`
	RefreshToastAutoDismiss     bool                    `json:"refresh_toast_auto_dismiss"`
	ArticleShareCount           int                     `json:"article_share_count"`
	ArticleShareSummaryLimit    int                     `json:"article_share_summary_limit"`
	WebImageFallback            bool                    `json:"web_image_fallback"`
	RemoteImages                string                  `json:"remote_images"`
	MediaCacheEnabled           bool                    `json:"media_cache_enabled"`
	Theme                       string                  `json:"theme"`
	ArticleViewMode             string                  `json:"article_view_mode"`
	ListDensity                 string                  `json:"list_density"`
	ListThumbnail               string                  `json:"list_thumbnail"`
	MasonryImageMode            string                  `json:"masonry_image_mode"`
	Animation                   string                  `json:"animation"`
	FollowSystemReduceMotion    bool                    `json:"follow_system_reduce_motion"`
	SidebarCollapsed            bool                    `json:"sidebar_collapsed"`
	SettingsSidebarCollapsed    bool                    `json:"settings_sidebar_collapsed,omitempty"`
	SettingsSidebarWidth        int                     `json:"settings_sidebar_width,omitempty"`
	SidebarWidth                int                     `json:"sidebar_width"`
	ListWidth                   int                     `json:"list_width"`
	ReaderLayout                string                  `json:"reader_layout"`
	ReaderPinned                bool                    `json:"reader_pinned"`
	ReaderOpen                  bool                    `json:"reader_open"`
	SelectedScopeID             string                  `json:"selected_scope_id"`
	SelectedScope               string                  `json:"selected_scope"`
	SelectedArticleID           string                  `json:"selected_article_id"`
	ScrollAnchors               map[string]ScrollAnchor `json:"scroll_anchors"`
	ReaderToolbarPinned         []string                `json:"reader_toolbar_pinned"`
	SettingsOpenMode            string                  `json:"settings_open_mode"`
	ReaderOpenMode              string                  `json:"reader_open_mode"`
	ReaderAutoRead              bool                    `json:"reader_auto_read"`
	ReaderFontFamily            string                  `json:"reader_font_family"`
	ReaderFontSize              int                     `json:"reader_font_size"`
	ReaderLineHeight            float64                 `json:"reader_line_height"`
	ReaderWidth                 int                     `json:"reader_width"`
	ReaderImageWheel            string                  `json:"reader_image_wheel"`
	ReaderImageBackdropClose    bool                    `json:"reader_image_backdrop_close"`
	ReaderCodeTheme             string                  `json:"reader_code_theme"`
	ChatCodeTheme               string                  `json:"chat_code_theme"`
	ChatModelID                 string                  `json:"chat_model_id"`
	ChatApprovalMode            string                  `json:"chat_approval_mode"`
	ChatApprovalPresentation    string                  `json:"chat_approval_presentation"`
	ChatBusySendMode            string                  `json:"chat_busy_send_mode"`
	ChatMaxRetries              int                     `json:"chat_max_retries"`
	ReaderOutline               string                  `json:"reader_outline"`
	ReaderTranslationDisplay    string                  `json:"reader_translation_display"`
	ListShowTranslated          bool                    `json:"list_show_translated"`
}

func Defaults() Preferences {
	return Preferences{RefreshIntervalMinutes: 30, NotificationMode: "toast", RefreshNotificationsEnabled: false, ArticleShareCount: 10, ArticleShareSummaryLimit: DefaultArticleShareSummaryLimit, ReaderLayout: "fill", ReaderOpenMode: "full", ReaderAutoRead: true, ReaderFontFamily: "sans", RemoteImages: "subscriptions", MediaCacheEnabled: true,
		Theme: "system", SettingsOpenMode: "full", ArticleViewMode: "masonry", ListDensity: "compact", ListThumbnail: "none",
		MasonryImageMode: "original_ratio", Animation: "full", FollowSystemReduceMotion: true,
		SidebarWidth: 248, SettingsSidebarWidth: 220, ListWidth: 360, ReaderOpen: false, SelectedScope: "all",
		ReaderToolbarPinned: []string{"refresh", "favorite", "translate", "summary", "ai"},
		ScrollAnchors:       map[string]ScrollAnchor{}, ReaderFontSize: 17, ReaderLineHeight: 1.7, ReaderWidth: 720,
		ReaderImageWheel: "zoom", ReaderImageBackdropClose: true, ReaderOutline: "ticks", ReaderCodeTheme: "app", ChatCodeTheme: "app",
		ChatApprovalMode: agentrun.ApprovalAsk, ChatApprovalPresentation: "composer", ChatBusySendMode: "queue", ChatMaxRetries: DefaultChatMaxRetries,
		ReaderTranslationDisplay: "bilingual", ListShowTranslated: true,
	}
}

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

var codeThemeID = regexp.MustCompile(`^(base16/)?[a-z0-9][a-z0-9-]{0,70}$`)

const (
	maxListWidth                    = 760
	maxChatModelID                  = 128
	MaxArticleShareCount            = 50
	DefaultArticleShareSummaryLimit = 50
	MaxArticleShareSummaryLimit     = 500
	DefaultChatMaxRetries           = 5
	MaxChatMaxRetries               = 10
)

func validReaderToolbarPinned(actions []string) bool {
	if len(actions) > 9 {
		return false
	}
	seen := make(map[string]bool, len(actions))
	for _, action := range actions {
		if seen[action] || !oneOf(action, "refresh", "favorite", "translate", "summary", "ai", "categories", "read", "view", "original") {
			return false
		}
		seen[action] = true
	}
	return true
}

func (p Preferences) Validate() error {
	if !agentrun.ValidApprovalMode(p.ChatApprovalMode) || !oneOf(p.ChatApprovalPresentation, "composer", "dialog") {
		return errors.New("聊天工具审批选项无效。")
	}
	if !oneOf(p.ChatBusySendMode, "queue", "steer") {
		return errors.New("聊天执行中发送模式无效。")
	}
	if p.ChatMaxRetries < 0 || p.ChatMaxRetries > MaxChatMaxRetries {
		return errors.New("聊天失败重试次数须为 0 至 10 次。")
	}
	if !codeThemeID.MatchString(p.ReaderCodeTheme) || !codeThemeID.MatchString(p.ChatCodeTheme) {
		return errors.New("代码块主题选项无效。")
	}
	if len(p.ChatModelID) > maxChatModelID {
		return errors.New("聊天模型标识无效。")
	}
	if !validReaderToolbarPinned(p.ReaderToolbarPinned) {
		return errors.New("阅读工具栏选项无效。")
	}
	if p.RefreshIntervalMinutes < 5 || p.RefreshIntervalMinutes > 1440 {
		return errors.New("订阅刷新间隔须为 5 至 1440 分钟。")
	}
	if !oneOf(p.NotificationMode, "toast", "native") {
		return errors.New("通知方式无效。")
	}
	if p.ArticleShareCount < 1 || p.ArticleShareCount > MaxArticleShareCount ||
		p.ArticleShareSummaryLimit < 1 || p.ArticleShareSummaryLimit > MaxArticleShareSummaryLimit {
		return errors.New("文章发送条数或摘要字数超出允许范围。")
	}
	if !oneOf(p.ReaderLayout, "fill", "drawer", "split") {
		return errors.New("阅读布局无效。")
	}
	if !oneOf(p.SettingsOpenMode, "full", "dialog") {
		return errors.New("设置打开方式无效。")
	}
	if !oneOf(p.ReaderOpenMode, "feed", "full") {
		return errors.New("文章打开方式无效。")
	}
	if !oneOf(p.ReaderFontFamily, "sans", "serif", "monospace") {
		return errors.New("阅读字体选项无效。")
	}
	if !oneOf(p.ReaderImageWheel, "zoom", "navigate") || !oneOf(p.ReaderOutline, "ticks", "auto", "fixed", "hidden") {
		return errors.New("看图或目录选项无效。")
	}
	if !oneOf(p.ReaderTranslationDisplay, "bilingual", "translated", "original") {
		return errors.New("阅读器翻译显示方式无效。")
	}
	if !oneOf(p.RemoteImages, "always", "subscriptions", "never") || !oneOf(p.Theme, "light", "dark", "system") || !oneOf(p.ArticleViewMode, "list", "masonry") ||
		!oneOf(p.ListDensity, "compact", "comfortable") || !oneOf(p.ListThumbnail, "none", "small") ||
		!oneOf(p.MasonryImageMode, "original_ratio", "cropped") || !oneOf(p.Animation, "full", "subtle", "off") {
		return errors.New("外观设置选项无效。")
	}
	if p.SettingsSidebarWidth < 160 || p.SettingsSidebarWidth > 300 || p.SidebarWidth < 220 || p.SidebarWidth > 360 || p.ListWidth < 300 || p.ListWidth > maxListWidth ||
		p.ReaderFontSize < 14 || p.ReaderFontSize > 24 || p.ReaderWidth < 680 || p.ReaderWidth > 760 ||
		math.IsNaN(p.ReaderLineHeight) || p.ReaderLineHeight < 1.4 || p.ReaderLineHeight > 2 {
		return errors.New("布局或阅读尺寸超出允许范围。")
	}
	if !oneOf(p.SelectedScope, "all", "unread", "favorites", "featured", "feed", "category") || ((p.SelectedScope == "feed" || p.SelectedScope == "category") && p.SelectedScopeID == "") || len(p.SelectedScopeID) > 256 || len(p.SelectedArticleID) > 256 || len(p.ScrollAnchors) > 32 {
		return errors.New("视图恢复设置无效。")
	}
	for key, anchor := range p.ScrollAnchors {
		if len(key) > 512 || len(anchor.ArticleID) > 256 || math.IsNaN(anchor.Offset) || math.IsInf(anchor.Offset, 0) || math.Abs(anchor.Offset) > 10_000_000 {
			return errors.New("浏览位置超出允许范围。")
		}
	}
	return nil
}

// normalize 只用于读取旧设置；非法字段分别回退，保留其他有效偏好。
func (p *Preferences) normalize() {
	d := Defaults()
	if !agentrun.ValidApprovalMode(p.ChatApprovalMode) {
		p.ChatApprovalMode = d.ChatApprovalMode
	}
	if !oneOf(p.ChatApprovalPresentation, "composer", "dialog") {
		p.ChatApprovalPresentation = d.ChatApprovalPresentation
	}
	if !oneOf(p.ChatBusySendMode, "queue", "steer") {
		p.ChatBusySendMode = d.ChatBusySendMode
	}
	if p.ChatMaxRetries < 0 || p.ChatMaxRetries > MaxChatMaxRetries {
		p.ChatMaxRetries = d.ChatMaxRetries
	}
	if !codeThemeID.MatchString(p.ReaderCodeTheme) {
		p.ReaderCodeTheme = d.ReaderCodeTheme
	}
	if !codeThemeID.MatchString(p.ChatCodeTheme) {
		p.ChatCodeTheme = d.ChatCodeTheme
	}
	if len(p.ChatModelID) > maxChatModelID {
		p.ChatModelID = d.ChatModelID
	}
	if p.ReaderToolbarPinned == nil || !validReaderToolbarPinned(p.ReaderToolbarPinned) {
		p.ReaderToolbarPinned = d.ReaderToolbarPinned
	}
	if p.RefreshIntervalMinutes < 5 || p.RefreshIntervalMinutes > 1440 {
		p.RefreshIntervalMinutes = d.RefreshIntervalMinutes
	}
	if !oneOf(p.NotificationMode, "toast", "native") {
		p.NotificationMode = d.NotificationMode
	}
	if p.ArticleShareCount < 1 || p.ArticleShareCount > MaxArticleShareCount {
		p.ArticleShareCount = d.ArticleShareCount
	}
	if p.ArticleShareSummaryLimit < 1 || p.ArticleShareSummaryLimit > MaxArticleShareSummaryLimit {
		p.ArticleShareSummaryLimit = d.ArticleShareSummaryLimit
	}
	if p.ReaderLayout == "" && p.ReaderPinned {
		p.ReaderLayout = "split"
	}
	if !oneOf(p.ReaderLayout, "fill", "drawer", "split") {
		p.ReaderLayout = d.ReaderLayout
	}
	if !oneOf(p.SettingsOpenMode, "full", "dialog") {
		p.SettingsOpenMode = d.SettingsOpenMode
	}
	if !oneOf(p.ReaderOpenMode, "feed", "full") {
		p.ReaderOpenMode = d.ReaderOpenMode
	}
	if !oneOf(p.ReaderFontFamily, "sans", "serif", "monospace") {
		p.ReaderFontFamily = d.ReaderFontFamily
	}
	if !oneOf(p.ReaderImageWheel, "zoom", "navigate") {
		p.ReaderImageWheel = d.ReaderImageWheel
	}
	if !oneOf(p.ReaderOutline, "ticks", "auto", "fixed", "hidden") {
		p.ReaderOutline = d.ReaderOutline
	}
	if !oneOf(p.ReaderTranslationDisplay, "bilingual", "translated", "original") {
		p.ReaderTranslationDisplay = d.ReaderTranslationDisplay
	}
	if !oneOf(p.RemoteImages, "always", "subscriptions", "never") {
		p.RemoteImages = d.RemoteImages
	}
	if !oneOf(p.Theme, "light", "dark", "system") {
		p.Theme = d.Theme
	}
	if !oneOf(p.ArticleViewMode, "list", "masonry") {
		p.ArticleViewMode = d.ArticleViewMode
	}
	if !oneOf(p.ListDensity, "compact", "comfortable") {
		p.ListDensity = d.ListDensity
	}
	if !oneOf(p.ListThumbnail, "none", "small") {
		p.ListThumbnail = d.ListThumbnail
	}
	if !oneOf(p.MasonryImageMode, "original_ratio", "cropped") {
		p.MasonryImageMode = d.MasonryImageMode
	}
	if !oneOf(p.Animation, "full", "subtle", "off") {
		p.Animation = d.Animation
	}
	if p.SettingsSidebarWidth < 160 || p.SettingsSidebarWidth > 300 {
		p.SettingsSidebarWidth = d.SettingsSidebarWidth
	}
	if p.SidebarWidth < 220 || p.SidebarWidth > 360 {
		p.SidebarWidth = d.SidebarWidth
	}
	if p.ListWidth < 300 || p.ListWidth > maxListWidth {
		p.ListWidth = d.ListWidth
	}
	if p.ReaderFontSize < 14 || p.ReaderFontSize > 24 {
		p.ReaderFontSize = d.ReaderFontSize
	}
	if p.ReaderWidth < 680 || p.ReaderWidth > 760 {
		p.ReaderWidth = d.ReaderWidth
	}
	if math.IsNaN(p.ReaderLineHeight) || p.ReaderLineHeight < 1.4 || p.ReaderLineHeight > 2 {
		p.ReaderLineHeight = d.ReaderLineHeight
	}
	if !oneOf(p.SelectedScope, "all", "unread", "favorites", "featured", "feed", "category") {
		p.SelectedScope = d.SelectedScope
	}
	if len(p.SelectedArticleID) > 256 {
		p.SelectedArticleID = ""
	}
	if len(p.SelectedScopeID) > 256 || ((p.SelectedScope == "feed" || p.SelectedScope == "category") && p.SelectedScopeID == "") {
		p.SelectedScope = "all"
		p.SelectedScopeID = ""
	}
	if p.SelectedScope != "feed" && p.SelectedScope != "category" {
		p.SelectedScopeID = ""
	}
	if p.ScrollAnchors == nil || len(p.ScrollAnchors) > 32 {
		p.ScrollAnchors = d.ScrollAnchors
	}
	for key, anchor := range p.ScrollAnchors {
		if len(key) > 512 || len(anchor.ArticleID) > 256 || math.IsNaN(anchor.Offset) || math.IsInf(anchor.Offset, 0) || math.Abs(anchor.Offset) > 10_000_000 {
			delete(p.ScrollAnchors, key)
		}
	}
}
