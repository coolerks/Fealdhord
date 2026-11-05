// Package desktop 装配原生窗口与菜单，业务命令统一交给前端执行器。
package desktop

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"runtime"
)

const CommandEventName = "fealdhord:command"

type CommandEvent struct {
	ID string `json:"id"`
}

func WindowOptions() application.WebviewWindowOptions {
	options := application.WebviewWindowOptions{
		Title: "折栈", Width: 1240, Height: 800, MinWidth: 900, MinHeight: 600, URL: "/",
		BackgroundType:   application.BackgroundTypeTranslucent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0), DefaultContextMenuDisabled: true,
		Mac: application.MacWindow{Backdrop: application.MacBackdropTranslucent, TitleBar: application.MacTitleBarHiddenInset, InvisibleTitleBarHeight: 52},
	}
	if runtime.GOOS != "darwin" {
		options.BackgroundType = application.BackgroundTypeSolid
		options.BackgroundColour = application.NewRGB(245, 245, 247)
	}
	return options
}

func InstallMenu(app *application.App) {
	if runtime.GOOS != "darwin" {
		return
	} // Windows／Linux 的菜单由前端 Menubar 实现。
	menu := application.NewMenu()
	add := func(target *application.Menu, label, id, key string) {
		item := target.Add(label).OnClick(func(*application.Context) { app.Event.Emit(CommandEventName, CommandEvent{ID: id}) })
		if key != "" {
			item.SetAccelerator(key)
		}
	}
	product := menu.AddSubmenu("折栈")
	add(product, "关于折栈", "about", "")
	add(product, "设置…", "settings", "Cmd+,")
	product.AddSeparator()
	product.AddRole(application.ServicesMenu)
	product.AddSeparator()
	product.AddRole(application.Hide).AddRole(application.HideOthers).AddRole(application.UnHide)
	product.AddSeparator()
	product.AddRole(application.Quit)
	file := menu.AddSubmenu("文件")
	add(file, "添加订阅…", "feeds", "Cmd+N")
	add(file, "订阅管理…", "feeds", "")
	file.Add("导入 OPML…").SetEnabled(false)
	file.Add("导出 OPML…").SetEnabled(false)
	file.AddSeparator()
	file.AddRole(application.CloseWindow)
	edit := menu.AddSubmenu("编辑")
	edit.AddRole(application.Undo).AddRole(application.Redo)
	edit.AddSeparator()
	edit.AddRole(application.Cut).AddRole(application.Copy).AddRole(application.Paste).AddRole(application.SelectAll)
	add(edit, "搜索文章", "search", "Cmd+F")
	view := menu.AddSubmenu("显示")
	add(view, "全部文章", "all", "Cmd+1")
	add(view, "未读文章", "unread", "Cmd+2")
	add(view, "收藏文章", "favorites", "Cmd+3")
	view.AddSeparator()
	add(view, "列表视图", "list", "")
	add(view, "瀑布流视图", "masonry", "")
	add(view, "显示／隐藏侧栏", "sidebar", "Cmd+Shift+L")
	add(view, "显示／隐藏阅读面板", "reader", "")
	add(view, "命令面板…", "palette", "Cmd+K")
	view.AddSeparator()
	add(view, "跟随系统外观", "theme-system", "")
	add(view, "浅色外观", "theme-light", "")
	add(view, "深色外观", "theme-dark", "")
	view.AddSeparator()
	view.AddRole(application.ToggleFullscreen)
	reading := menu.AddSubmenu("文章")
	add(reading, "上一篇", "previous", "")
	add(reading, "下一篇", "next", "")
	reading.AddSeparator()
	add(reading, "收藏／取消收藏", "favorite", "")
	add(reading, "切换已读状态", "read", "")
	window := menu.AddSubmenu("窗口")
	window.AddRole(application.Minimise).AddRole(application.Zoom)
	help := menu.AddSubmenu("帮助")
	add(help, "键盘快捷键与帮助", "help", "")
	add(help, "诊断与版本信息", "about", "")

	labels := map[application.Role]string{application.Undo: "撤销", application.Redo: "重做", application.Cut: "剪切", application.Copy: "复制", application.Paste: "粘贴", application.SelectAll: "全选", application.Quit: "退出折栈", application.Hide: "隐藏折栈", application.HideOthers: "隐藏其他", application.UnHide: "显示全部", application.ServicesMenu: "服务", application.CloseWindow: "关闭窗口", application.ToggleFullscreen: "切换全屏", application.Minimise: "最小化", application.Zoom: "缩放窗口"}
	for role, label := range labels {
		if item := menu.FindByRole(role); item != nil {
			item.SetLabel(label)
		}
	}
	app.Menu.SetApplicationMenu(menu)
}
