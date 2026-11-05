// Package settings 保存本地界面偏好，不保存文章或凭据。
package settings

import (
	"errors"
	"math"
)

type ScrollAnchor struct {
	ArticleID string  `json:"article_id"`
	Offset    float64 `json:"offset"`
}

type Preferences struct {
	WebImageFallback         bool                    `json:"web_image_fallback"`
	RemoteImages             string                  `json:"remote_images"`
	MediaCacheEnabled        bool                    `json:"media_cache_enabled"`
	Theme                    string                  `json:"theme"`
	ArticleViewMode          string                  `json:"article_view_mode"`
	ListDensity              string                  `json:"list_density"`
	ListThumbnail            string                  `json:"list_thumbnail"`
	MasonryImageMode         string                  `json:"masonry_image_mode"`
	Animation                string                  `json:"animation"`
	FollowSystemReduceMotion bool                    `json:"follow_system_reduce_motion"`
	SidebarCollapsed         bool                    `json:"sidebar_collapsed"`
	SidebarWidth             int                     `json:"sidebar_width"`
	ListWidth                int                     `json:"list_width"`
	ReaderPinned             bool                    `json:"reader_pinned"`
	ReaderOpen               bool                    `json:"reader_open"`
	SelectedScopeID          string                  `json:"selected_scope_id"`
	SelectedScope            string                  `json:"selected_scope"`
	SelectedArticleID        string                  `json:"selected_article_id"`
	ScrollAnchors            map[string]ScrollAnchor `json:"scroll_anchors"`
	ReaderFontSize           int                     `json:"reader_font_size"`
	ReaderLineHeight         float64                 `json:"reader_line_height"`
	ReaderWidth              int                     `json:"reader_width"`
}

func Defaults() Preferences {
	return Preferences{RemoteImages: "subscriptions", MediaCacheEnabled: true,
		Theme: "system", ArticleViewMode: "list", ListDensity: "compact", ListThumbnail: "none",
		MasonryImageMode: "original_ratio", Animation: "full", FollowSystemReduceMotion: true,
		SidebarWidth: 248, ListWidth: 360, ReaderOpen: false, SelectedScope: "all",
		ScrollAnchors: map[string]ScrollAnchor{}, ReaderFontSize: 17, ReaderLineHeight: 1.7, ReaderWidth: 720,
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

func (p Preferences) Validate() error {
	if !oneOf(p.RemoteImages, "always", "subscriptions", "never") || !oneOf(p.Theme, "light", "dark", "system") || !oneOf(p.ArticleViewMode, "list", "masonry") ||
		!oneOf(p.ListDensity, "compact", "comfortable") || !oneOf(p.ListThumbnail, "none", "small") ||
		!oneOf(p.MasonryImageMode, "original_ratio", "cropped") || !oneOf(p.Animation, "full", "subtle", "off") {
		return errors.New("外观设置选项无效。")
	}
	if p.SidebarWidth < 220 || p.SidebarWidth > 280 || p.ListWidth < 300 || p.ListWidth > 420 ||
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
	if p.SidebarWidth < 220 || p.SidebarWidth > 280 {
		p.SidebarWidth = d.SidebarWidth
	}
	if p.ListWidth < 300 || p.ListWidth > 420 {
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
